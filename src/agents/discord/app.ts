import { Bus, KVNotFound } from '../../clients/typescript/src/index.ts';
import { Subscriptions, type Caller } from '../shared/subscriptions.ts';
import { Router } from '../shared/router.ts';
import { Api, chunks } from './api.ts';
import { configuration, snowflake, type Config } from './config.ts';
import { parse } from './parse.ts';
import { Store } from './store.ts';
export type CallerMessage = Caller & {message_id:string;topic?:string;tag?:string};
export class App {
  readonly subscriptions: Subscriptions;
  readonly router: Router<string>;
  private store: Store;
  private tail: Promise<unknown> = Promise.resolve();
  private bot?: {id:string;username?:string};
  private seen = new Set<string>();
  private parts: Record<string,{root:string;until:number}> = Object.create(null);
  private stopped = false;
  constructor(readonly bus:Bus,readonly name:string,readonly api:Api,private rawConfig:()=>Promise<unknown>,private log:(line:string)=>void) {
    this.store=new Store(bus,name);
    this.subscriptions=new Subscriptions(this.store,name,{
      validateSelector:s=>{if(Object.keys(s).join()!=='channel'||!snowflake(s.channel))throw Error('channel selector required');},
      selectorMatches:(s,e)=>s.channel===e.channel,
      warning:(code,id)=>log(`subscription ${id??'store'}: ${code}`),
    });
    const kv=()=>bus.record(name).kv;
    this.router=new Router<string>({self:name,platform:{label:'Discord',name:'Discord',topic:'discord-router'},
      get:async()=>{try{return String((await kv().get('string','router')).value);}catch(e){if(e instanceof KVNotFound)return undefined;throw e;}},
      set:async value=>{await kv().set('string','router',JSON.stringify(value),{mode:'set'});},
      config:async()=>this.config(),
      send:async(route,body,reply_to)=>(await bus.request('POST','/send',{...route,body,reply_to})).message_id,
      tell:async(channel,reference,text)=>{
        await this.channel(channel,await this.config());
        const ids=await this.post(channel,text,reference),root=ids.at(-1)!;
        for(const id of ids.slice(0,-1))this.parts[channel+':'+id]={root,until:Date.now()+7*86400000};
        this.parts=Object.fromEntries(Object.entries(this.parts).filter(([,v])=>v.until>Date.now()).slice(-1000));
        await kv().set('json','router.parts',this.parts as any,{mode:'set'});
        return root;
      },fallback:async()=>false,log,
    });
  }
  async config():Promise<Config>{return configuration(await this.rawConfig());}
  async initialize(){await this.config();await this.api.credentials();await this.identity();await this.router.load();try{const raw=(await this.bus.record(this.name).kv.get('json','router.parts')).value;if(raw&&typeof raw==='object'&&!Array.isArray(raw))for(const [key,v]of Object.entries(raw)){const p=v as any;if(typeof p?.root==='string'&&snowflake(p.root)&&Number.isFinite(p.until)&&p.until>Date.now())this.parts[key]=p;}}catch(e){if(!(e instanceof KVNotFound))throw e;}await this.subscriptions.reload();}
  private turn<T>(run:()=>Promise<T>):Promise<T>{const p=this.tail.then(async()=>{if(this.stopped)throw Error('agent is stopping');this.store.deadline=Date.now()+15000;this.api.deadline=this.store.deadline;await this.subscriptions.reload();return run();});this.tail=p.catch(()=>{});return p;}
  private async identity(){if(!this.bot){const bot=await this.api.call('GET','/users/@me');if(!snowflake(bot?.id))throw Error('Discord returned no bot identity');this.bot={id:bot.id,username:bot.username};}return this.bot;}
  private async available(cfg:Config){const rows:any[]=[];for(const guild of cfg.guilds){const result=await this.api.call('GET',`/guilds/${guild}/channels`);if(!Array.isArray(result))throw Error('Discord returned no channel list');rows.push(...result.filter(c=>c.guild_id===guild&&[0,5,10,11,12].includes(c.type)));}return rows;}
  async channel(value:string|undefined,cfg:Config):Promise<any>{
    value=value??cfg.default;if(!value)throw Error('Configuration default is required when no channel is given');
    let id=cfg.channels[value]?.id;
    if(!id&&snowflake(value))id=value;
    if(!id&&value.startsWith('#')){const matches=(await this.available(cfg)).filter(c=>c.name===value!.slice(1));if(matches.length!==1)throw Error('channel name must resolve exactly once in configured guilds');id=matches[0].id;}
    if(!snowflake(id))throw Error('unknown channel; configure its alias and id');
    const channel=await this.api.call('GET',`/channels/${id}`);
    if(channel?.id!==id||![0,1,5,10,11,12].includes(channel.type))throw Error('unsupported Discord channel');
    if(channel.guild_id!==undefined&&!cfg.guilds.includes(channel.guild_id))throw Error('channel guild is outside Configuration guilds');
    if(channel.guild_id===undefined&&channel.type!==1)throw Error('channel has no allowed guild');
    return channel;
  }
  private async post(channel:string,text:string,reply_to?:string,deadline=Date.now()+15000){const ids:string[]=[];for(const content of chunks(text)){const r=await this.api.call('POST',`/channels/${channel}/messages`,{content,allowed_mentions:{parse:[],replied_user:false},...(reply_to?{message_reference:{message_id:reply_to,fail_if_not_exists:true}}:{})},deadline);if(!snowflake(r?.id))throw Error('Discord returned no message id');ids.push(r.id);}return ids;}
  call(body:string,caller:CallerMessage):Promise<unknown>{return this.turn(async()=>{
    const cfg=await this.config(),c=parse(body,Object.keys(cfg.channels));
    if(c.method==='help')return {help:'Use help-json for declared methods.'};
    if(c.method==='whoami'){const bot=await this.identity();const all=await this.api.call('GET','/users/@me/guilds');if(!Array.isArray(all))throw Error('Discord returned no guild list');return {...bot,guilds:all.filter(g=>cfg.guilds.includes(g.id)).map(g=>({id:g.id,name:g.name}))};}
    if(c.method==='channels'){const all=await this.available(cfg);return all.filter(x=>!c.filter||String(x.name??'').toLowerCase().includes(c.filter.toLowerCase())).map(x=>({id:x.id,name:x.name,type:x.type,guild_id:x.guild_id,aliases:Object.entries(cfg.channels).filter(([,v])=>v.id===x.id).map(([k])=>k)}));}
    if(c.method==='sub.list')return this.subscriptions.list(caller);
    if(c.method==='sub.delete')return {deleted:await this.subscriptions.delete(caller,c.id!)};
    const channel=await this.channel(c.channel,cfg),id=channel.id;
    if(c.method==='channel')return {id,name:channel.name,type:channel.type,topic:channel.topic??''};
    if(c.method==='sub.add')return this.subscriptions.add(caller,c.target!,{channel:id},c.filter??'');
    if(c.method==='read'){const messages=await this.api.call('GET',`/channels/${id}/messages?limit=${c.limit}`);if(!Array.isArray(messages))throw Error('Discord returned no messages');return messages.slice().reverse().map(m=>({id:m.id,author:m.author?.id,text:m.content??'',reply_to:m.message_reference?.message_id}));}
    if(c.method==='post'){
      const tag=await this.router.tag(caller.from,cfg),ids=await this.post(id,`${c.text}\n\n#${tag}`,c.reply_to,this.store.deadline);
      for(const mid of ids)await this.router.posted(id,mid,{to:caller.from,topic:caller.topic,tag:caller.tag,re:caller.message_id});
      return {channel:id,id:ids.at(-1),ids};
    }
    if(c.method==='react'){await this.api.call('PUT',`/channels/${id}/messages/${c.id}/reactions/${encodeURIComponent(c.emoji!)}/@me`);return {reacted:true};}
    if(c.method==='delete'){await this.api.call('DELETE',`/channels/${id}/messages/${c.id}`);return {deleted:true};}
    if(c.method==='edit'){
      const previous=await this.api.call('GET',`/channels/${id}/messages/${c.id}`);
      if(previous?.author?.id!==(await this.identity()).id)throw Error('only the bot may edit its own message text');
      if([...c.text!].length>2000)throw Error('an edited message is at most 2000 characters');
      await this.api.call('PATCH',`/channels/${id}/messages/${c.id}`,{content:c.text,allowed_mentions:{parse:[]}});return {edited:true};
    }
    if(c.method==='thread'){const r=await this.api.call('POST',`/channels/${id}/messages/${c.id}/threads`,{name:c.name});return {id:r?.id,name:r?.name};}
    throw Error('unknown method');
  });}
  event(kind:string,m:any):Promise<void>{return this.turn(async()=>{
    if(kind==='READY'&&snowflake(m?.user?.id)){this.bot={id:m.user.id,username:m.user.username};return;}
    if(kind!=='MESSAGE_CREATE'||!snowflake(m?.id)||!snowflake(m?.channel_id)||!snowflake(m?.author?.id))return;
    const cfg=await this.config();if(m.guild_id!==undefined&&!cfg.guilds.includes(m.guild_id))return;
    if(m.author.id===(await this.identity()).id)return;
    const channel=await this.channel(m.channel_id,cfg);
    const key=channel.id+':'+m.id;if(this.seen.has(key))return;this.seen.add(key);if(this.seen.size>1000)this.seen.delete(this.seen.values().next().value!);
    const text=typeof m.content==='string'?m.content:'';
    const ref=m.message_reference?.message_id,reply=ref?(this.parts[channel.id+':'+ref]?.root??ref):undefined;
    const mapped=reply&&this.router.mapped(channel.id,reply),mention=`<@${this.bot!.id}>`,nickMention=`<@!${this.bot!.id}>`;
    if(channel.type===1||mapped||text.includes(mention)||text.includes(nickMention))await this.router.incoming({message_id:m.id,from_id:m.author.id,username:m.author.username,text:text.split(mention).join('').split(nickMention).join('').trim(),...(mapped?{reply_to:reply}:{}),date:0},channel.id,channel.name,cfg);
    const body=JSON.stringify({guild:m.guild_id,channel:channel.id,user:m.author.id,text,message_id:m.id,...(ref?{reply_to:ref}:{})});
    await this.subscriptions.deliver({channel:channel.id},text,body,{topic:'discord',tag:channel.id,deadline:this.store.deadline});
  });}
  answer(message:any){return this.turn(()=>this.router.answer(message));}
  async close(){this.stopped=true;this.api.close();this.subscriptions.close();let timer:ReturnType<typeof setTimeout>;try{await Promise.race([this.tail,new Promise<void>(resolve=>{timer=setTimeout(resolve,5000);})]);}finally{clearTimeout(timer!);}}
}
