import { expect, test } from "bun:test";
import { slackSecret } from "./secrets.ts";
test("only current credential keys are accepted, without exposing values", () => {
  const parse=(text: string)=>slackSecret(new TextEncoder().encode(text));
  const raw="SLACK_BOT_TOKEN=bot\nSLACK_APP_TOKEN=app\nSLACK_API_URL=https://slack.com/api/\n";
  expect(parse(raw)).toEqual({SLACK_BOT_TOKEN:"bot",SLACK_APP_TOKEN:"app",SLACK_API_URL:"https://slack.com/api/"});
  for (const key of ["SLACK_USERNAME","SLACK_ICON","UNKNOWN"]) {
    expect(() => parse(raw+`${key}=private-value\n`)).toThrow("unsupported key");
    try { parse(raw+`${key}=private-value\n`); } catch(e) { expect(String(e)).not.toContain("private-value"); }
  }
});
