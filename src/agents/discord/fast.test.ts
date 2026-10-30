// Fast essentials only: no server, worker, wait or provider/bus connection.
import { expect,test } from 'bun:test';
import { configuration } from './config.ts';
import { parse } from './parse.ts';
import { chunks } from './api.ts';
import { helpData } from '../shared/metadata.ts';
import manifest from './agent.json';
const envDist=await Bun.file(new URL('./env.dist',import.meta.url)).text();
test('Discord fast manifest help declares the implemented calls and Secret names without values',()=>{
 const help:any=helpData(manifest,'help-json',envDist);
 expect(Object.keys(help.methods).sort()).toEqual(['channel','channels','delete','edit','post','react','read','sub.add','sub.delete','sub.list','thread','whoami'].sort());
 expect(help.secrets.map((s:any)=>s.name)).toEqual(['DISCORD_BOT_TOKEN','DISCORD_API_URL','DISCORD_GATEWAY_URL']);
 expect(help.methods['sub.edit']).toBeUndefined();
});
test('Discord fast config requires alias ids and refuses duplicate destinations',()=>{
 expect(()=>configuration({guilds:['1'],channels:{alerts:{}}})).toThrow('needs id');
 expect(()=>configuration({guilds:['1'],channels:{alerts:{id:'10'},'#alerts':{id:'10'}}})).toThrow('alerts and #alerts');
 expect(configuration({guilds:['1'],channels:{'10':{tags:false}}}).channels['10'].id).toBe('10');
});
test('Discord fast parsing preserves a named channel and rejects removed sub editing',()=>{
 expect(parse('post alerts hello',['alerts'])).toEqual({method:'post',channel:'alerts',text:'hello'});
 expect(parse('read #alerts 50')).toEqual({method:'read',channel:'#alerts',limit:50});
 expect(()=>parse('sub edit 123')).toThrow('delete it and add it again');
});
test('Discord fast splitting bounds provider posts and truncates the eighth part',()=>{
 const out=chunks('x'.repeat(20000));expect(out).toHaveLength(8);expect(out.every(s=>[...s].length<=2000)).toBe(true);expect(out.at(-1)).toEndWith('… (truncated)');
});
