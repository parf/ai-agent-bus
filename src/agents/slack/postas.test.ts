import { expect, test } from "bun:test";
import { postAs } from "./postas.ts";

// The name and icon a post goes under: call, then configuration, then the bus
// secret, then cli@<host>; each layer seen to win over the next.
test("a post's name and icon come from the call, the configuration, the secret, then cli@<host>", () => {
  const host = "parf.us";
  expect(postAs({}, {}, {}, host)).toEqual({ username: "cli@parf" });
  expect(postAs({}, {}, { SLACK_USERNAME: "s", SLACK_ICON: ":s:" }, host)).toEqual({ username: "s", icon_emoji: ":s:" });
  expect(postAs({}, { SLACK_USERNAME: "c@{host}", SLACK_ICON: ":c:" }, { SLACK_USERNAME: "s", SLACK_ICON: ":s:" }, host)).toEqual({ username: "c@parf", icon_emoji: ":c:" });
  expect(postAs({ username: "x", icon: "https://e/i.png" }, { SLACK_USERNAME: "c", SLACK_ICON: ":c:" }, {}, host)).toEqual({ username: "x", icon_url: "https://e/i.png" });
  expect(postAs({}, { SLACK_USERNAME: "" }, { SLACK_USERNAME: "s" }, host)).toEqual({});
});
