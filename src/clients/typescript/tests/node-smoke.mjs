import assert from 'node:assert/strict';
import { spawn, spawnSync, execFileSync } from 'node:child_process';
import { mkdtempSync,readFileSync,rmSync } from 'node:fs';
import { join,resolve,dirname } from 'node:path';
import { fileURLToPath,pathToFileURL } from 'node:url';
import { createServer } from 'node:https';
import { createHash,X509Certificate } from 'node:crypto';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../../../..');
const bundle=resolve(process.argv[2]);
execFileSync(process.execPath,['--input-type=module','-e',"import { Bus, VERSION } from '@agent-bus/client'; if (typeof Bus.connect !== 'function' || !VERSION) throw new Error('package exports broken');"],{cwd:dirname(bundle),stdio:'inherit'});
const {Bus,BusUnreachable,VERSION}=await import(pathToFileURL(bundle).href);
const scratch=mkdtempSync(join(root,'tmp/ts-node-'));
const processFixture=spawn(process.env.AB_CLIENT_FIXTURE ?? join(root,'tmp/client-fixture'),[join(scratch,'owner.sock')],{stdio:'pipe'});
const info=await new Promise((resolve,reject)=>{let text='';const timer=setTimeout(()=>reject(new Error('fixture startup timed out')),3000);processFixture.stdout.on('data',chunk=>{text+=chunk;if(text.includes('\n')){clearTimeout(timer);resolve(JSON.parse(text.split('\n')[0]));}});processFixture.on('error',reject);});
const buses=[];let server,serving;
try {
 assert.equal(VERSION,readFileSync(join(root,'src/internal/version/VERSION'),'utf8').trim());
 const owner=Bus.connect({addr:join(scratch,'owner.sock'),token:'',expectName:'owner@fixture'});buses.push(owner);assert.equal((await owner.whoami()).you,'owner@fixture');
 const caller=Bus.connect({addr:info.addr,token:info.tokens['#caller@fixture'],expectName:'#caller@fixture'}),worker=Bus.connect({addr:info.addr,token:info.tokens['#worker@fixture'],expectName:'#worker@fixture'});buses.push(caller,worker);
 const kv=owner.record('jobs@fixture').kv;await kv.set('int','exact',9223372036854775807n,{mode:'set'});assert.equal((await kv.get('int','exact')).value,9223372036854775807n);
 serving=worker.serve(async msg=>`node:${msg.body}`,{drainTimeout:1000});const answer=await caller.call('#worker@fixture','smoke',{timeout:1000});assert.equal(answer.kind,'answer');assert.equal(answer.message.body,'node:smoke');worker.stop();await serving;
 const key=join(scratch,'key.pem'),cert=join(scratch,'cert.pem');assert.equal(spawnSync('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-days','1','-subj','/CN=localhost','-keyout',key,'-out',cert],{stdio:'ignore'}).status,0);
 let credentials=0;server=createServer({key:readFileSync(key),cert:readFileSync(cert)},(req,res)=>{if(req.headers['x-agent-bus-token'])credentials++;res.end('{"you":"#caller@fixture"}');});await new Promise(r=>server.listen(0,'127.0.0.1',r));const addr=`https://127.0.0.1:${server.address().port}`;
 const wrong=Bus.connect({addr,token:'no-leak',fingerprint:'00'.repeat(32)});buses.push(wrong);await assert.rejects(wrong.whoami(),BusUnreachable);assert.equal(credentials,0);
 const pin=createHash('sha256').update(new X509Certificate(readFileSync(cert)).raw).digest('hex');const right=Bus.connect({addr,token:'dummy',fingerprint:pin});buses.push(right);assert.equal((await right.whoami()).you,'#caller@fixture');assert.equal(credentials,1);
 console.log('Node package smoke: HTTP calls, Unix identity, exact int64, pin-before-credentials passed');
} finally {
 for(const bus of buses){try{bus.stop();}catch{}await bus.close();}await serving?.catch(()=>{});
 if(server){server.closeAllConnections();await new Promise(r=>server.close(r));}
 const exit=new Promise(r=>processFixture.once('exit',r));processFixture.stdin.end();await exit;rmSync(scratch,{recursive:true,force:true});
}
