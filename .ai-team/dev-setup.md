# AB-DVP dev setup (local, never released)

The team's AI sessions on this node are windows (tabs) of ONE tmux session `AB-DVP` (`tmux attach -t AB-DVP`), started by the instance `#tmux-sessions/ab-dvp@parf.us` (owner, 2026-10-27); the main `#tmux-sessions@parf.us` stays unconfigured.

| window | bus name | command | cwd |
|---|---|---|---|
| AB | `#claude/ab` | `ab-claude` | `/usr/local/src/ai-agent-bus` |
| AB2 | `#claude/ab2` | `ab-claude -2` | `/usr/local/src/ai-agent-bus` |
| xAB | `#codex/xab` | `ab-codex` | `/usr/local/src/ai-agent-bus` |
| oAB | `#opencode/oab` | `ab-opencode` | `/usr/local/src/ai-agent-bus` |
| xAB2 | `#codex/xab2` | `ab-codex` | `/usr/local/src/ai-agent-bus/Plans` |
| AB2-review | `#claude/ab2-review` | `ab-claude -2` + Claude sandbox `.ai-team/review-settings.json` + no Edit/Write (reviewer, no FS writes) | `/usr/local/src/ai-agent-bus/Plans` |

They replace `#claude/ab-dvp` (Claude A), `#claude/ab-dvp-b` (Claude B), `#codex/ab-dvp` (Codex #2) and `#codex/ab-dvp-3` (Codex #3).

## Configuration

Validate it against the RT28 parser before saving it.

```json
{"session":"AB-DVP","windows":[
 {"name":"AB","cwd":"/usr/local/src/ai-agent-bus","env":{"AGENT_BUS_NAME":"#claude/ab"},"command":"ab-claude --restart --tag ab-dvp"},
 {"name":"AB2","cwd":"/usr/local/src/ai-agent-bus","env":{"AGENT_BUS_NAME":"#claude/ab2"},"command":"ab-claude -2 --restart --tag ab-dvp"},
 {"name":"xAB","cwd":"/usr/local/src/ai-agent-bus","env":{"AGENT_BUS_NAME":"#codex/xab"},"command":"ab-codex --restart --tag ab-dvp"},
 {"name":"oAB","cwd":"/usr/local/src/ai-agent-bus","env":{"AGENT_BUS_NAME":"#opencode/oab"},"command":"ab-opencode --restart --tag ab-dvp"},
 {"name":"xAB2","cwd":"/usr/local/src/ai-agent-bus/Plans","env":{"AGENT_BUS_NAME":"#codex/xab2"},"command":"ab-codex --restart --tag ab-dvp"},
 {"name":"AB2-review","cwd":"/usr/local/src/ai-agent-bus/Plans","env":{"AGENT_BUS_NAME":"#claude/ab2-review"},"command":"ab-claude -2 --restart --tag ab-dvp --settings /usr/local/src/ai-agent-bus/.ai-team/review-settings.json --disallowedTools 'Edit,Write,NotebookEdit,Bash(git push:*),Bash(git commit:*),Bash(sudo:*)'"}
]}
```

## Switchover

1. Before the switch, every place that names the old sessions moves to the new names:
   - CLAUDE.md `smoke-gate` Maintainers;
   - the `lock://smoke-gate` record's Maintainers;
   - the Claude memory files `team-agents` and `contact-only-ab-dvp`.
2. The owner saves the Configuration: `agent-bus config '#tmux-sessions/ab-dvp@parf.us' -` < the JSON above, or on the web.
3. The owner ends the old sessions.
4. `agent-bus call '#tmux-sessions/ab-dvp@parf.us' start`, then `agent-bus ls --tag=ab-dvp` lists five live agents.
5. Remove the old session records.

## Daemon rebuild (after RT29 lands)

The owner chooses the records to keep (Slack, Telegram, the AI sessions, …). Then:

1. Full stopped backup of the daemon.
2. `agent-bus-admin export` of the chosen records.
3. Recreate the daemon database.
4. `agent-bus-admin import`.
5. Check each agent with `check-deploy` and `ls --all`.

Runner agents come back from their host files (`runner/` env and setup). Their bus-side Configuration and Secret come back only if their records are exported too.
Credentials have to carry over, or every agent holding a token stops working: see RT29.
The session-name switchover above can happen in the same window.


## Local Qdrant (RD131, 2026-10-28)

This is parf's rootless Podman deployment, not a bus component. Owner approval:
AB2 message `892218ddec928eb0`; it supersedes all earlier rootful setup approval.
Linger readback was `yes`; it was not changed. Podman is 5.8.9; uid is 1001.

Exact operator commands used (from the repository root):

```sh
id
loginctl show-user parf --property=Linger --value
podman --version
podman container exists qdrant
podman volume exists qdrant-data
python3 .ai-team/qdrant-node.py
python3 .ai-team/qdrant-node-finish.py
XDG_RUNTIME_DIR=/run/user/1001 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus systemctl --user show qdrant.service --property=ActiveState,SubState,WantedBy
podman info --format '{{.Host.Security.Rootless}}'
podman port qdrant
podman inspect --format '{{.State.Status}} {{.Config.Image}}' qdrant
podman volume inspect --format '{{.Name}}' qdrant-data
```

The first setup run started the service but its first HTTP probe saw a transient
connection reset before the runner-key write. The finish command retried bounded
GET checks and performed the single root operation. The setup script now catches
that startup reset. Both exact command scripts are reproduced below, without key
values. The only `sudo` is the stdin-only runner-key writer; no bus unit restarts.

Deployment readback:

- `qdrant.service`: active/running, `WantedBy=default.target`; linger=yes unchanged.
- Rootless=true, named volume `qdrant-data`, image digest pinned to v1.20.0.
- Mapping `6333/tcp -> 127.0.0.1:6333`.
- Authenticated `/collections`: HTTP200, empty collection list; server version1.20.0.
- No-key `/collections`: HTTP401.
- `~/.config/qdrant/env`: 0600, key never printed. Runner assignment at
  `/var/lib/agent-bus/parf-ab-runner/qdrant/env`: runner-owned0600, equality verified.
- Reboot startup is configured via user Quadlet+linger; no actual reboot was run.

The proxy URL for this node will be `http://127.0.0.1:6333` in R.4; this is a local
choice, not a default imposed by the bus. Container key is `QDRANT__SERVICE__API_KEY`;
runner key is `QDRANT_API_KEY`; they hold the same generated bytes.

Setup-only binary promotion used:

```sh
mkdir -p tmp/r3-node-build
BUILD_COMMIT=24d276ce src/build.sh /usr/local/src/ai-agent-bus/tmp/r3-node-build
install -m 0755 tmp/r3-node-build/agent-bus-setup src/agent-bus-setup
src/agent-bus-setup --version
src/check-deploy.sh
```

Only the stamped setup binary was promoted (0.9.127, build24d276ce); other build
outputs were discarded. `check-deploy` passed six checks. No daemon/runner/web
restart, rootful Podman, firewall/sysctl change, prune, uninstall or push.

<details><summary>Exact local setup script</summary>

```python
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
```

</details>

<details><summary>Exact finish/check and stdin writer invocation</summary>

```python
import ast, os, json, subprocess, time, urllib.request, urllib.error
from pathlib import Path
keyfile=Path.home()/'.config/qdrant/env'
key=keyfile.read_text().strip().split('=',1)[1]
assert len(key)==64 and all(c in '0123456789abcdef' for c in key)
client=urllib.request.build_opener(urllib.request.ProxyHandler({}))
end=time.monotonic()+30
while True:
    try:
        req=urllib.request.Request('http://127.0.0.1:6333/collections',headers={'api-key':key})
        with client.open(req,timeout=2) as response:
            assert response.status==200
            data=json.loads(response.read(1<<20))
            print('authenticated collections HTTP200; collection count',len(data['result']['collections']),flush=True)
        break
    except (urllib.error.URLError,OSError,TimeoutError):
        if time.monotonic()>=end: raise RuntimeError('bounded readiness failed') from None
        time.sleep(.2)
try:
    client.open('http://127.0.0.1:6333/collections',timeout=2).close()
except urllib.error.HTTPError as e:
    assert e.code in (401,403)
    print('no-key collections refused HTTP'+str(e.code),flush=True)
else: raise RuntimeError('no-key request allowed')
req=urllib.request.Request('http://127.0.0.1:6333/',headers={'api-key':key})
with client.open(req,timeout=2) as response:
    print('Qdrant version',json.loads(response.read(4096))['version'],flush=True)
module=ast.parse(Path('.ai-team/qdrant-node.py').read_text())
writer=next(ast.literal_eval(n.value) for n in module.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='writer' for t in n.targets))
subprocess.run(['sudo','-n','python3','-c',writer],input=('QDRANT_API_KEY='+key+'\n').encode(),check=True)
```

</details>


## Qdrant bus proxy (R.4, 2026-10-28)

Shared release0.9.130, committed `ff8ffffb`. Manager routine-deploy approval
`92a265d930381fab`, rebased same-file approval `43e45a771c4eebdd`.
The real rootful setup component remains removed. The operator-run rootless
store and its private key were neither changed nor restarted by this deploy.

Exact host copy (the committed temporary worktree was removed after deployment):

```sh
sudo -n /usr/local/src/ai-agent-bus/tmp/qdrant-proxy/src/agents/qdrant/package.sh /var/lib/agent-bus/agent.d/qdrant
```

Only `qdrant.py`0755 and `agent.json`, `env.dist`, `LICENSE.md`,
`support/agents/shared/{metadata.py,provider_http.py}` and
`support/internal/version/VERSION`0644 were installed. No host secret was read
or written. For a future redeploy use the reviewed committed checkout's package
script, preserving the same destination and private host env.

As Userparf on the login socket: runner calls `reload`, `enable qdrant`, `reload`;
then `/configure` set `#qdrant@parf.us` to exactly
`{"url":"http://127.0.0.1:6333"}`. The newly created record had management-only
empty ACL; it was set explicitly to `@owner` before any API data call. Supported
roles read/write/admin were registered. No runner/daemon service restart was
performed here; AB2 separately deployed the RT32 docs loader to runner0.9.130.

Live acceptance16/16: a unique `ab_r4_acceptance_` synthetic collection and a
temporary caller proved owner admin, no-role write/notadmin, read query/notwrite,
write upsert/notadmin, mixed read+bare stillread, unknown path needsadmin,
admin unknownpath reaches Qdrant404 and outside-API refusal. Finally removed
only that prefix-guarded collection, the caller and its added ACL terms;
ACL returned to `@owner`. Existing collections were not compared globally or
changed. No-key store check remains R.3's HTTP401; proxy API key was not exposed.

`src/check-deploy.sh` passed6/6. Normal gate1209/0 in49.131s;25owningmutants
assertion-red; reviewed source and deployed entry SHA match. RT32 owns adding
its `docs` manifest URL after its loader; the guide already follows all10
required headings. No push, container change, reboot or new host key operation.
