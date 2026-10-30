import { expect, test } from "bun:test";
import { pluginSteps } from "./plugin-steps.ts";

const add = (p: string) => ["marketplace", "add", p];
const install = ["install", "agent-bus@agent-bus-local", "--scope", "user"];
const uninstall = ["uninstall", "agent-bus@agent-bus-local"];
const remove = ["marketplace", "remove", "agent-bus-local"];

test("a fresh Claude home gets the marketplace and the plugin, compiled or not", () => {
  for (const compiled of [true, false]) expect(pluginSteps({ compiled, source: "/r/1", installed: false })).toEqual([add("/r/1"), install]);
});

test("a release replaces a plugin another release or a checkout installed", () => {
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "/rel/0.9.163", installed: true })).toEqual([uninstall, remove, add("/rel/0.9.164"), install]);
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.163", knownPath: "/usr/local/src/ab/src", installed: true })).toEqual([uninstall, remove, add("/rel/0.9.163"), install]);
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "/rel/0.9.163", installed: false })).toEqual([remove, add("/rel/0.9.164"), install]);
});

test("the same release, and any checkout, leave an installed plugin alone", () => {
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "/rel/0.9.164", installed: true })).toEqual([]);
  expect(pluginSteps({ compiled: false, source: "/tmp/worktree/src", knownPath: "/usr/local/src/ab/src", installed: true })).toEqual([]);
  expect(pluginSteps({ compiled: false, source: "/usr/local/src/ab/src", knownPath: "/usr/local/src/ab/src", installed: false })).toEqual([install]);
});
