import { parseBusName, userName, agentName } from '../shared/names.ts';
import { shortTag, type Config as Routing } from '../shared/router.ts';
export type Channel = { id: string; route?: string; tags?: boolean; anyone?: boolean };
export type Config = Routing & { guilds: string[]; channels: Record<string, Channel>; default?: string };
export const snowflake = (s: unknown): s is string => typeof s === 'string' && /^[0-9]{1,20}$/.test(s);
const object = (x: unknown): x is Record<string, any> => !!x && typeof x === 'object' && !Array.isArray(x);
export function configuration(raw: unknown): Config {
  if (!object(raw)) throw Error('Configuration must be an object with guilds');
  for (const key of Object.keys(raw)) if (!['guilds','users','tags','route','channels','default'].includes(key)) throw Error(`Configuration: unknown ${key}`);
  if (!Array.isArray(raw.guilds) || raw.guilds.some(g => !snowflake(g)) || new Set(raw.guilds).size !== raw.guilds.length) throw Error('Configuration guilds must be distinct server ids');
  const cfg: Config = { guilds: [...raw.guilds], channels: Object.create(null), chats: Object.create(null) };
  if (raw.default !== undefined) { if (typeof raw.default !== 'string' || !raw.default) throw Error('Configuration default must name a channel'); cfg.default = raw.default; }
  if (raw.route !== undefined) { if (!parseBusName(raw.route)) throw Error('Configuration route must be a bus name'); cfg.route = parseBusName(raw.route); }
  for (const field of ['users','tags'] as const) {
    if (raw[field] === undefined) continue;
    if (!object(raw[field])) throw Error(`Configuration ${field} must be an object`);
    cfg[field] = Object.create(null);
    const seen = new Set<string>();
    for (const [key,value] of Object.entries(raw[field])) {
      if (field === 'users' ? !snowflake(key) || !userName.test(value) : !shortTag.test(key) || !agentName.test(value)) throw Error(`Configuration ${field}: invalid ${key}`);
      const name = parseBusName(value)!;
      if (field === 'tags' && seen.has(name)) throw Error(`Configuration tags: repeated session ${name}`);
      cfg[field]![key] = name; seen.add(name);
    }
  }
  if (raw.channels !== undefined && !object(raw.channels)) throw Error('Configuration channels must be an object');
  const ids = new Map<string,string>();
  for (const [key,value] of Object.entries(raw.channels ?? {})) {
    if (!key || /[\s\x00-\x1f]/.test(key) || !object(value)) throw Error(`Configuration channels: invalid ${key}`);
    for (const field of Object.keys(value)) if (!['id','route','tags','anyone'].includes(field)) throw Error(`Configuration channels: unknown ${key}.${field}`);
    const id = Object.hasOwn(value,'id') ? value.id : (snowflake(key) ? key : undefined);
    if (!snowflake(id)) throw Error(`Configuration channels: ${key} needs id`);
    if (snowflake(key) && key !== id) throw Error(`Configuration channels: ${key} disagrees with id ${id}`);
    const other = ids.get(id); if (other) throw Error(`Configuration channels: ${other} and ${key} resolve to the same id ${id}`);
    ids.set(id,key);
    if (value.route !== undefined && !parseBusName(value.route)) throw Error(`Configuration channels: ${key}.route is not a bus name`);
    for (const f of ['tags','anyone']) if (value[f] !== undefined && typeof value[f] !== 'boolean') throw Error(`Configuration channels: ${key}.${f} must be boolean`);
    const entry: Channel = { ...value, id, ...(value.route === undefined ? {} : {route:parseBusName(value.route)!}) };
    cfg.channels[key] = entry; cfg.chats![id] = { ...(entry.route === undefined ? {} : {route:entry.route}), ...(entry.tags === undefined ? {} : {tags:entry.tags}), ...(entry.anyone === undefined ? {} : {anyone:entry.anyone}) };
  }
  return cfg;
}
