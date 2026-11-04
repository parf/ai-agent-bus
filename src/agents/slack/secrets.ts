import { parseEnvFile } from "../shared/envfile.ts";

/** Only provider credentials and its endpoint belong in the record Secret. */
export function slackSecret(raw: Uint8Array): Record<string, string> {
  const value = parseEnvFile("the bus secret", raw);
  if (Object.keys(value).some(key => !["SLACK_BOT_TOKEN", "SLACK_APP_TOKEN", "SLACK_API_URL"].includes(key)))
    throw Error("the bus secret contains an unsupported key; post appearance belongs in the runner environment");
  return value;
}
