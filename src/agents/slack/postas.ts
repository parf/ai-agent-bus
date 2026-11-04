// Post appearance: call override, runner environment, then the default name.
/** Slack's chat.postMessage fields for a post's name and icon. */
export function postAs(
  call: { username?: string; icon?: string },
  config: Record<string, string | undefined>,
  host: string,
): Record<string, string> {
  const out: Record<string, string> = {};
  const name = call.username ?? config.SLACK_USERNAME ?? "cli@{host}";
  if (name !== "") out.username = name.replaceAll("{host}", host.match(/^[\w-]*/)![0]);
  const icon = call.icon ?? config.SLACK_ICON;
  if (icon) out[icon.startsWith("http://") || icon.startsWith("https://") ? "icon_url" : "icon_emoji"] = icon;
  return out;
}
