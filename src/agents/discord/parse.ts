const names = ['post','read','channels','channel','react','edit','delete','thread','whoami','sub.add','sub.list','sub.delete','help'];
export type Call = { method: string; channel?: string; text?: string; id?: string; emoji?: string; name?: string; reply_to?: string; limit?: number; filter?: string; target?: string };
export function parse(body: string, aliases: string[] = []): Call {
  let call: any;
  if (body.trim().startsWith('{')) { try { call = JSON.parse(body); } catch { throw Error('invalid JSON call'); } }
  else {
    const words = body.trim().split(/\s+/); let method = words.shift() ?? '';
    if (method === 'sub') method += '.' + (words.shift() ?? '');
    if(method==='sub.edit')throw Error('delete it and add it again');
    call = { method };
    const channelLike = (s?: string) => !!s && (s.startsWith('#') || aliases.includes(s) || /^[0-9]{1,20}$/.test(s));
    if (method === 'post') { if (words.length > 1 && channelLike(words[0])) call.channel = words.shift(); call.text = words.join(' '); }
    else if (method === 'read') {
      if (words.length && (words[0]!.startsWith('#') || aliases.includes(words[0]!) || words.length > 1 || Number(words[0]) > 100)) call.channel = words.shift();
      if (words.length) call.limit = Number(words.shift());
    } else if (method === 'channels') call.filter = words.join(' ');
    else if (method === 'channel') call.channel = words.shift();
    else if (['react','edit','delete','thread'].includes(method)) { call.channel = words.shift(); call.id = words.shift(); if (method === 'react') call.emoji = words.join(' '); if (method === 'edit') call.text = words.join(' '); if (method === 'thread') call.name = words.join(' '); words.length = 0; }
    else if (method === 'sub.add') { call.channel = words.shift(); call.target = words.shift(); call.filter = words.join(' '); words.length = 0; }
    else if (method === 'sub.delete') call.id = words.shift();
    if (!['post','channels'].includes(method) && words.length) throw Error('too many arguments');
  }
  if (!call || typeof call !== 'object' || Array.isArray(call) || typeof call.method !== 'string') throw Error('expected a Discord call');
  if (call.method === 'sub.edit') throw Error('delete it and add it again');
  if (!names.includes(call.method)) throw Error('unknown method; use help');
  const fields: Record<string,string[]> = { post:['channel','text','reply_to'], read:['channel','limit'], channels:['filter'], channel:['channel'], react:['channel','id','emoji'], edit:['channel','id','text'], delete:['channel','id'], thread:['channel','id','name'], whoami:[], 'sub.add':['channel','target','filter'], 'sub.list':[], 'sub.delete':['id'], help:[] };
  for (const key of Object.keys(call)) if (key !== 'method' && !fields[call.method]!.includes(key)) throw Error(`unexpected argument ${key}`);
  for (const [key,value] of Object.entries(call)) if (key !== 'limit' && typeof value !== 'string') throw Error(`${key} must be text`);
  if (call.method === 'read') { call.limit ??= 50; if (!Number.isInteger(call.limit) || call.limit < 1 || call.limit > 100) throw Error('limit must be 1..100'); }
  for (const field of ({post:['text'], channel:['channel'],react:['channel','id','emoji'],edit:['channel','id','text'],delete:['channel','id'],thread:['channel','id','name'],'sub.add':['channel','target'],'sub.delete':['id']} as Record<string,string[]>)[call.method] ?? []) if (!call[field]?.trim()) throw Error(`${field} is required`);
  if (call.reply_to !== undefined && !/^[0-9]{1,20}$/.test(call.reply_to)) throw Error('reply_to must be a message id');
  if (['react','edit','delete','thread'].includes(call.method) && !/^[0-9]{1,20}$/.test(call.id)) throw Error('id must be a message id');
  if (call.method === 'thread' && [...call.name].length > 100) throw Error('thread name is at most 100 characters');
  return call;
}
