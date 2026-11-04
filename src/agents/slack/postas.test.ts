import { expect, test } from "bun:test";
import { postAs } from "./postas.ts";

test("post appearance uses call overrides, then the runner environment", () => {
  const host = "node.example";
  expect(postAs({}, {}, host)).toEqual({ username: "cli@node" });
  expect(postAs({}, {SLACK_USERNAME:"service@{host}",SLACK_ICON:":robot_face:"}, host)).toEqual({username:"service@node",icon_emoji:":robot_face:"});
  expect(postAs({username:"override",icon:"https://example.org/icon.png"},{SLACK_USERNAME:"service",SLACK_ICON:":robot_face:"},host)).toEqual({username:"override",icon_url:"https://example.org/icon.png"});
  expect(postAs({username:""},{SLACK_USERNAME:"service"},host)).toEqual({});
});
