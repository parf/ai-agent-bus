"""Throwaway databases for the proxy tests: MariaDB from the installed server, PostgreSQL and ClickHouse in rootless podman.

Everything listens on loopback or a private socket and is removed on exit. The passwords are made-up test values.
"""
import atexit
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

POSTGRES = 'docker.io/library/postgres@sha256:ef257d85f76e48da1c64832459b59fcaba1a4dac97bf5d7450c77753542eee94'
CLICKHOUSE = 'docker.io/clickhouse/clickhouse-server@sha256:87e0a5b72f5465b18eacca7c76850e7ff551c9795c50e451f5646299e5e24146'
SENTINEL = 'sentinel-pw-7f3a91'   # the one password every fixture account uses
STOP = []
PRIVATE = []   # this run's private temp dir: the log copies and the ClickHouse config; removed with the run
START_SECONDS = 45   # per attempt; two attempts make the 90 s a start used to be given


@atexit.register
def _cleanup():
    for fn in reversed(STOP):
        try:
            fn()
        except Exception:
            pass
    STOP.clear()
    for path in PRIVATE:
        shutil.rmtree(path, ignore_errors=True)
    PRIVATE.clear()


def _terminate(number, frame):
    sys.exit(128 + number)   # a signal's default ends the process without atexit: this makes it run the cleanup


try:
    for _number in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
        signal.signal(_number, _terminate)
except ValueError:   # imported off the main thread: no handlers, the watchdog still covers the run
    pass


def private():
    """The run's own temp dir, under the repository's tmp/ and never /tmp."""
    if not PRIVATE:
        scratch = Path(__file__).resolve().parents[3] / 'tmp'
        scratch.mkdir(exist_ok=True)
        PRIVATE.append(tempfile.mkdtemp(prefix='ab-db-fixture-', dir=scratch))
    return PRIVATE[0]


WATCH = 'while kill -0 "$1" 2>/dev/null; do sleep 1; done; shift; exec "$@"'


def watch(*cleanup):
    """Run `cleanup` once this process is gone, however it went: SIGKILL runs no handler and no atexit.

    The watcher is its own session, so a kill aimed at the test's process group leaves it alone.
    """
    subprocess.Popen(['sh', '-c', WATCH, 'sh', str(os.getpid()), *cleanup], start_new_session=True,
                     stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def wait(probe, what, seconds=60):
    end = time.time() + seconds
    while time.time() < end:
        try:
            if probe():
                return
        except Exception:
            pass
        time.sleep(0.3)
    raise RuntimeError(f'{what} did not start')


def mariadb():
    """Config for a fresh MariaDB on a unix socket, with accounts app (all), ro, rw and adm."""
    scratch = Path(__file__).resolve().parents[3] / 'tmp'   # never /tmp
    scratch.mkdir(exist_ok=True)
    root = tempfile.mkdtemp(prefix='ab-db-maria-', dir=scratch)
    data, sock = os.path.join(root, 'data'), os.path.join(root, 'sock')
    subprocess.run(['mariadb-install-db', '--no-defaults', f'--datadir={data}', '--auth-root-authentication-method=normal',
                    '--skip-test-db'], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    server = subprocess.Popen(['mariadbd', '--no-defaults', f'--datadir={data}', f'--socket={sock}', '--skip-networking',
                               '--innodb-buffer-pool-size=16M', '--performance-schema=OFF', f'--pid-file={root}/pid'],
                              stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    STOP.append(lambda: (server.terminate(), server.wait(20), shutil.rmtree(root, ignore_errors=True)))
    watch('sh', '-c', 'kill "$1" 2>/dev/null; sleep 3; rm -rf "$2"', 'sh', str(server.pid), root)
    import pymysql

    def admin():
        return pymysql.connect(unix_socket=sock, user='root', autocommit=True)
    wait(lambda: admin().close() or True, 'MariaDB')
    c = admin().cursor()
    for sql in ['CREATE DATABASE app',
                'CREATE TABLE app.t (id INT PRIMARY KEY, v VARCHAR(20))',
                "INSERT INTO app.t VALUES (1,'one'),(2,'two')",
                f"CREATE USER 'app'@'localhost' IDENTIFIED BY '{SENTINEL}'", "GRANT ALL ON app.* TO 'app'@'localhost'",
                "GRANT ALL ON *.* TO 'app'@'localhost'",
                f"CREATE USER 'ro'@'localhost' IDENTIFIED BY '{SENTINEL}'", "GRANT SELECT ON app.* TO 'ro'@'localhost'",
                f"CREATE USER 'rw'@'localhost' IDENTIFIED BY '{SENTINEL}'",
                "GRANT SELECT,INSERT,UPDATE,DELETE ON app.* TO 'rw'@'localhost'",
                f"CREATE USER 'adm'@'localhost' IDENTIFIED BY '{SENTINEL}'", "GRANT ALL ON app.* TO 'adm'@'localhost'",
                "CREATE FUNCTION app.sneaky() RETURNS INT MODIFIES SQL DATA BEGIN INSERT INTO app.t VALUES (99,'sneaky'); RETURN 1; END"]:
        c.execute(sql)
    return {'engine': 'mysql', 'socket': sock, 'database': 'app'}


def podman(*args, check=True):
    return subprocess.run(['podman', *args], check=check, capture_output=True, text=True)


def container(image, port, env, files=()):
    """Start a container with its port on a free loopback port; return (name, host port).

    `files` are (host path, container path) pairs mounted read-only. The container's own log is capped, so a chatty
    server cannot fill the disk; a watcher removes the container if this process dies without cleaning up.
    """
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        host_port = s.getsockname()[1]
    for stale in leaked():   # a killed earlier run's container, cleared before it can hold a port or a disk
        podman('rm', '-f', '-t', '1', stale, check=False)
    name = f'ab-db-fixture-{os.getpid()}-{host_port}'
    # No --rm: a container that exits must stay listed for diagnose, which copies its logs out before bring_up and STOP remove it.
    # The label scopes this run's containers: diagnostics and cleanup name only these, never a neighbour's.
    args = ['run', '-d', '--name', name, '--label', f'ab-db-fixture={os.getpid()}', '-p', f'127.0.0.1:{host_port}:{port}',
            '--log-driver', 'k8s-file', '--log-opt', 'max-size=10mb']
    for host, inside in files:
        args += ['-v', f'{host}:{inside}:ro,Z']
    for key, value in env.items():
        args += ['-e', f'{key}={value}']
    # Registered before the run: a run that fails after creating the container still leaves one to remove.
    STOP.append(lambda: podman('rm', '-f', '-t', '1', name, check=False))
    watch('podman', 'rm', '-f', '-t', '1', name)
    podman(*args, image)
    return name, host_port


def speaks_postgres(port):
    """True once something on the port answers PostgreSQL's SSL request with its one byte, S or N.

    The published port accepts connections the moment the container exists, so a connect proves nothing;
    only the server's own answer does, and asking costs the container nothing, unlike podman exec.
    """
    with socket.create_connection(('127.0.0.1', port), timeout=2) as s:
        s.sendall(bytes([0, 0, 0, 8, 4, 210, 22, 47]))
        return s.recv(1) in (b'S', b'N')


def diagnose(what):
    """Say what this run's containers look like and what they last said, so a failed start explains itself."""
    mine = f'label=ab-db-fixture={os.getpid()}'
    print(f'fixture {what} failed; containers of this run:', file=sys.stderr)
    print(podman('ps', '-a', '--filter', mine, '--format', '{{.Names}} {{.Status}}', check=False).stdout, file=sys.stderr)
    for line in podman('ps', '-a', '--filter', mine, '--format', '{{.Names}}', check=False).stdout.split():
        # Copied into the run's private dir first, so the container can be removed and the lines are still there.
        copy = Path(private()) / f'{line}.log'
        copy.write_text(podman('logs', '--tail', '20', line, check=False).stdout)
        print(f'--- last lines of {line}', file=sys.stderr)
        print(copy.read_text(), file=sys.stderr)


def bring_up(what, make, ready, setup, seconds=None):
    """Start a container, wait for it, set it up; from scratch once more if it never got there or vanished.

    Two attempts of 45 s are the 90 s one used to be. A container another rootless podman user removed or
    starved is the case this is for (J25): the second attempt starts a fresh one, and a second failure says
    what the containers were doing.
    """
    first = last = None
    seconds = START_SECONDS if seconds is None else seconds
    for _ in (1, 2):
        name, port = make()
        try:
            wait(lambda: ready(name, port), what, seconds)
            setup(name, port)
            return name, port
        except (RuntimeError, subprocess.CalledProcessError) as error:
            first = first or error
            last = error
            diagnose(what)
            podman('rm', '-f', '-t', '1', name, check=False)
    raise RuntimeError(f'{what} did not start in two attempts: first {first!r}, last {last!r}')


def postgres():
    def ready(name, port):
        return speaks_postgres(port) and podman('exec', name, 'psql', '-U', 'postgres', '-d', 'app', '-c', 'select 1',
                                               check=False).returncode == 0

    def setup(name, port):
        def psql(sql, user='postgres'):
            return podman('exec', name, 'psql', '-U', user, '-d', 'app', '-v', 'ON_ERROR_STOP=1', '-c', sql)
        for sql in ['CREATE TABLE t (id INT PRIMARY KEY, v TEXT)', "INSERT INTO t VALUES (1,'one'),(2,'two')",
                    f"CREATE ROLE ro LOGIN PASSWORD '{SENTINEL}'", f"CREATE ROLE rw LOGIN PASSWORD '{SENTINEL}'",
                    f"CREATE ROLE adm LOGIN PASSWORD '{SENTINEL}'", f"CREATE ROLE app LOGIN PASSWORD '{SENTINEL}'",
                    'GRANT ALL ON t TO app', 'GRANT ALL ON SCHEMA public TO app',
                    'GRANT SELECT ON t TO ro', 'GRANT SELECT,INSERT,UPDATE,DELETE ON t TO rw', 'GRANT ALL ON SCHEMA public TO adm',
                    'GRANT ALL ON t TO adm',
                    'CREATE FUNCTION sneaky() RETURNS int LANGUAGE sql AS $$ INSERT INTO t VALUES (99,\'sneaky\') RETURNING 1 $$',
                    'GRANT EXECUTE ON FUNCTION sneaky() TO ro, rw, adm, app',
                    'CREATE EXTENSION IF NOT EXISTS dblink']:
            psql(sql)
    _, port = bring_up('PostgreSQL', lambda: container(POSTGRES, 5432, {'POSTGRES_PASSWORD': SENTINEL, 'POSTGRES_DB': 'app'}),
                       ready, setup)
    # 'postgres' is a superuser; it is the account the guide warns about.
    return {'engine': 'postgres', 'host': '127.0.0.1', 'port': port, 'database': 'app'}


LOGGING = """<clickhouse><logger><level>warning</level><console>1</console><log remove="remove"/><errorlog remove="remove"/></logger></clickhouse>
"""   # the image logs to /var/log/clickhouse-server without a limit: stdout only, which the container's log cap bounds


def leaked():
    """Names of fixture containers whose test process is gone: what a run that was killed left behind."""
    names = podman('ps', '-a', '--filter', 'label=ab-db-fixture', '--format', '{{.Names}} {{.Labels}}', check=False).stdout
    out = []
    for line in names.splitlines():
        name = line.split()[0]
        pid = name.split('-')[3] if name.count('-') >= 4 else '0'
        if not os.path.exists(f'/proc/{pid}'):
            out.append(name)
    return out


def clickhouse():
    logging = os.path.join(private(), 'logging.xml')
    Path(logging).write_text(LOGGING)

    def sql(name, text):
        return podman('exec', name, 'clickhouse-client', '--user', 'app', '--password', SENTINEL, '-q', text)

    def ready(name, port):
        return sql(name, "SELECT count() FROM system.databases WHERE name = 'app'").stdout.strip() == '1'

    def setup(name, port):
        time.sleep(2)   # the image's own bootstrap server restarts once
        for text in ['CREATE TABLE app.t (id UInt32, v String) ENGINE = MergeTree ORDER BY id',
                     "INSERT INTO app.t VALUES (1,'one'),(2,'two')",
                     f"CREATE USER ro IDENTIFIED BY '{SENTINEL}' SETTINGS readonly = 1", 'GRANT SELECT ON app.* TO ro',
                     f"CREATE USER rw IDENTIFIED BY '{SENTINEL}'", 'GRANT SELECT, INSERT, ALTER UPDATE, ALTER DELETE ON app.* TO rw',
                     f"CREATE USER adm IDENTIFIED BY '{SENTINEL}'", 'GRANT ALL ON app.* TO adm']:
            sql(name, text)
    _, port = bring_up('ClickHouse', lambda: container(CLICKHOUSE, 8123, {'CLICKHOUSE_USER': 'app', 'CLICKHOUSE_PASSWORD': SENTINEL,
                                                                         'CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT': '1', 'CLICKHOUSE_DB': 'app'},
                                                                         files=[(logging, '/etc/clickhouse-server/config.d/logging.xml')]),
                       ready, setup)
    return {'engine': 'clickhouse', 'host': '127.0.0.1', 'port': port, 'database': 'app'}


if __name__ == '__main__' and sys.argv[1:] == ['leaked']:
    print('\n'.join(leaked()))
