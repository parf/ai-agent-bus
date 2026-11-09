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
