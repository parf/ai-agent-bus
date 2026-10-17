// The name and icon a post goes under (Plans/R1.2/slack/README.md § behaviour
// pinned by the review, Posting): the call's own, else the configuration's —
// the runner's env layers, which win over the bus secret as local wins over
// remote — else the bus secret's, else slack-post's cli@<host>. `{host}` in a
// name is the host's short name. An empty name posts as the bot itself.

/** Slack's chat.postMessage fields for a post's name and icon. */
export function postAs(
  call: { username?: string; icon?: string },
  config: Record<string, string | undefined>,
  secret: Record<string, string | undefined>,
  host: string,
): Record<string, string> {
  const out: Record<string, string> = {};
  const name = call.username ?? config.SLACK_USERNAME ?? secret.SLACK_USERNAME ?? "cli@{host}";
  if (name !== "") out.username = name.replaceAll("{host}", host.match(/^[\w-]*/)![0]);
  const icon = call.icon ?? config.SLACK_ICON ?? secret.SLACK_ICON;
  if (icon) out[icon.startsWith("http://") || icon.startsWith("https://") ? "icon_url" : "icon_emoji"] = icon;
  return out;
}
