#!/usr/bin/env python3
"""One node's owner-approved commands (RD131); not a bus installer."""
import os, pwd, secrets, subprocess, time, urllib.request, urllib.error
from pathlib import Path

assert pwd.getpwuid(os.getuid()).pw_name == 'parf'
image = 'docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2'
home = Path.home()
unitdir = home / '.config/containers/systemd'
keydir = home / '.config/qdrant'
keyfile = keydir / 'env'
container = unitdir / 'qdrant.container'
volume = unitdir / 'qdrant-data.volume'
env = dict(os.environ, XDG_RUNTIME_DIR=f'/run/user/{os.getuid()}', DBUS_SESSION_BUS_ADDRESS=f'unix:path=/run/user/{os.getuid()}/bus')

def run(*argv):
    return subprocess.run(argv, env=env, check=True)

linger = subprocess.check_output(['loginctl', 'show-user', 'parf', '--property=Linger', '--value'], text=True).strip()
print('linger:', linger, '(unchanged)', flush=True)
assert linger == 'yes'
for path in (keyfile, container, volume):
    if path.exists() or path.is_symlink():
        raise RuntimeError(f'refusing existing local deployment file: {path}')
for kind, name in [('container', 'qdrant'), ('volume', 'qdrant-data')]:
    rc = subprocess.run(['podman', kind, 'exists', name], env=env).returncode
    if rc != 1:
        raise RuntimeError(f'refusing existing or unreadable {kind}: {name} (rc={rc})')
loaded = subprocess.check_output(['systemctl', '--user', 'show', 'qdrant.service', '--property=LoadState', '--value'], env=env, text=True).strip()
if loaded != 'not-found':
    raise RuntimeError('refusing existing qdrant.service: ' + loaded)

keydir.mkdir(parents=True, mode=0o700, exist_ok=True)
os.chmod(keydir, 0o700)
unitdir.mkdir(parents=True, exist_ok=True)
key = secrets.token_hex(32)
fd = os.open(keyfile, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
with os.fdopen(fd, 'w') as f:
    f.write('QDRANT__SERVICE__API_KEY=' + key + '\n'); f.flush(); os.fsync(f.fileno())
container.write_text(f'''[Unit]
Description=Local operator-run Qdrant
[Container]
Image={image}
Pull=never
ContainerName=qdrant
PublishPort=127.0.0.1:6333:6333
Volume=qdrant-data.volume:/qdrant/storage
EnvironmentFile={keyfile}
[Service]
Restart=on-failure
RestartSec=3
TimeoutStartSec=120
[Install]
WantedBy=default.target
''')
volume.write_text('[Volume]\nVolumeName=qdrant-data\n')
os.chmod(container, 0o644); os.chmod(volume, 0o644)
print('local env file0600 and user Quadlets written; key not displayed', flush=True)
run('podman', 'pull', image)
run('systemctl', '--user', 'daemon-reload')
run('systemctl', '--user', 'start', 'qdrant.service')
run('systemctl', '--user', 'is-active', 'qdrant.service')
client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
end = time.monotonic() + 120
while True:
    try:
        req = urllib.request.Request('http://127.0.0.1:6333/collections', headers={'api-key': key})
        with client.open(req, timeout=2) as response:
            assert response.status == 200
            response.read(1 << 20)
        break
    except (urllib.error.URLError, OSError, TimeoutError):
        if time.monotonic() >= end:
            raise RuntimeError('Qdrant readiness deadline') from None
        time.sleep(.2)
try:
    client.open('http://127.0.0.1:6333/collections', timeout=2).close()
except urllib.error.HTTPError as error:
    assert error.code in (401, 403)
    print('no-key /collections refused HTTP', error.code, flush=True)
else:
    raise RuntimeError('Qdrant allowed no-key request')

# The only sudo operation: receive the runner assignment through stdin.
writer = '''import os,pwd,sys
v=sys.stdin.buffer.read(256)
if not v.startswith(b"QDRANT_API_KEY=") or len(v)!=80 or v[-1:]!=b"\\n" or any(c not in b"0123456789abcdef" for c in v[15:-1]):
 raise SystemExit("invalid key assignment (value hidden)")
u=pwd.getpwnam("parf-ab-runner")
p=os.open("/var/lib/agent-bus/parf-ab-runner",os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
created=False
try: os.mkdir("qdrant",0o700,dir_fd=p);created=True
except FileExistsError: pass
d=os.open("qdrant",os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=p)
if created: os.fchown(d,u.pw_uid,u.pw_gid);os.fchmod(d,0o700)
s=os.fstat(d)
if s.st_uid!=u.pw_uid or s.st_mode&0o777!=0o700: raise SystemExit("unsafe runner directory")
f=os.open("env",os.O_RDWR|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=d)
os.fchown(f,u.pw_uid,u.pw_gid);os.fchmod(f,0o600)
if os.write(f,v)!=len(v): raise SystemExit("short key write")
os.fsync(f);os.lseek(f,0,0)
if os.read(f,256)!=v: raise SystemExit("key write verification failed")
s=os.fstat(f)
if s.st_uid!=u.pw_uid or s.st_mode&0o777!=0o600: raise SystemExit("unsafe key permissions")
os.close(f);os.fsync(d);os.close(d);os.close(p)
print("runner key written via stdin; runner-owned0600; equality verified without display")
'''
# QDRANT_API_KEY= has 15 bytes, then 64 hex bytes and a newline.
assert len(('QDRANT_API_KEY=' + key + '\n').encode()) == 80
subprocess.run(['sudo', '-n', 'python3', '-c', writer], input=('QDRANT_API_KEY=' + key + '\n').encode(), env=env, check=True)
print('rootless deployment complete; no bus services restarted', flush=True)
