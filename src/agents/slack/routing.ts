import { Router, agentName, shortTag, type Config, type RouterDeps } from "../shared/router.ts";
export type RoutingConfig = Config & { enabled: boolean };
import { busName, userName, parseBusName } from "../shared/names.ts";
const object = (value: unknown): value is Record<string, unknown> => !!value && typeof value === "object" && !Array.isArray(value);
/** Slack keeps its exact timestamp strings; channel names are resolved once per snapshot. */
export async function routingConfig(raw: unknown, channelId: (name: string) => Promise<string | undefined>): Promise<RoutingConfig> {
  if (raw == null) return { enabled: false };
  if (!object(raw)) throw Error("the configuration is not an object");
  const config: RoutingConfig = { enabled: ["users", "tags", "route", "channels"].some(key => Object.hasOwn(raw, key)) };
  for (const key of Object.keys(raw)) if (!["users", "tags", "route", "channels"].includes(key)) throw Error(`${key} is not a configuration key`);
  if (Object.hasOwn(raw, "route")) {
    if (typeof raw.route !== "string" || !busName.test(raw.route)) throw Error("route is not a bus name");
    config.route = parseBusName(raw.route)!;
  }
  for (const field of ["users", "tags"] as const) {
    if (!Object.hasOwn(raw, field)) continue;
    if (!object(raw[field])) throw Error(`${field} is not an object`);
    const values = Object.create(null) as Record<string, string>, seen = new Set<string>();
    for (const [key, value] of Object.entries(raw[field])) {
      if (typeof value !== "string" || (field === "users" ? !/^[UW][A-Z0-9]+$/.test(key) || !userName.test(value) : !shortTag.test(key) || !agentName.test(value))) throw Error(`${field}: ${key} is invalid`);
      if (field === "tags" && seen.has(parseBusName(value)!)) throw Error(`tags: ${value} has more than one tag`);
      seen.add(parseBusName(value)!); values[key] = parseBusName(value)!;
    }
    config[field] = values;
  }
  if (Object.hasOwn(raw, "channels")) {
    if (!object(raw.channels)) throw Error("channels is not an object");
    config.chats = Object.create(null);
    for (const [key, value] of Object.entries(raw.channels)) {
      if (!/^(?:[CDG][A-Z0-9]+|#[a-z0-9_-]+)$/.test(key)) throw Error(`channels: ${key} is not a channel id or #name`);
      if (!object(value)) throw Error(`channels: ${key} is not an object`);
      for (const option of Object.keys(value)) if (!["route", "tags", "anyone"].includes(option)) throw Error(`channels: ${key}.${option} is unknown`);
      if (Object.hasOwn(value, "route") && (typeof value.route !== "string" || !busName.test(value.route))) throw Error(`channels: ${key}.route is not a bus name`);
      for (const option of ["tags", "anyone"]) if (Object.hasOwn(value, option) && typeof value[option] !== "boolean") throw Error(`channels: ${key}.${option} is not a boolean`);
      const id = await channelId(key);
      if (id === undefined) continue;
      if (Object.hasOwn(config.chats!, id)) throw Error(`channels: ${key} repeats channel ${id}`);
      config.chats![id] = { ...value, ...(value.route === undefined ? {} : {route:parseBusName(value.route)!}) };
    }
  }
  return config;
}

export class SlackRouter extends Router<string> {
  constructor(private readonly slack: Omit<RouterDeps<string>, "fallback" | "platform"> & { config: () => Promise<RoutingConfig>; person?: (id: string) => Promise<{ name?: string; real_name?: string } | undefined> }) {
    super({ ...slack, fallback: async () => false, platform: { label: "Slack", name: "Slack", topic: "slack-router" } });
  }
  /** Routing never gates subscriptions, and ordinary channel chatter is silent. */
  async event(event: any, botUser?: string, title?: string): Promise<void> {
    const config = await this.slack.config();
    if (!config.enabled) return;
    const text = typeof event.text === "string" ? event.text : "";
    const thread = typeof event.thread_ts === "string" ? event.thread_ts : undefined;
    const mapped = thread !== undefined && this.mapped(event.channel, thread);
    const mention = botUser ? `<@${botUser}>` : undefined;
    if (event.channel_type !== "im" && !event.channel.startsWith("D") && !(mention && text.includes(mention)) && !mapped) return;
    const person = typeof event.user === "string" ? await this.slack.person?.(event.user).catch(() => undefined) : undefined;
    await this.incoming({ message_id: event.ts, answer_to: thread ?? event.ts, from_id: event.user ?? event.bot_id, username: event.username ?? person?.name, name: event.user_profile?.real_name ?? person?.real_name,
      text: mention ? text.split(mention).join("").trim() : text, ...(mapped ? { reply_to: thread } : {}), date: 0 }, event.channel, title, config);
  }
}
