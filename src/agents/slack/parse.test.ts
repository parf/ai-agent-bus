import { describe, expect, test } from "bun:test";
import { parseCall } from "./parse.ts";

// S.2: the text and JSON forms parse to the same call, and anything unknown
// or malformed answers help (Plans/R1.2/slack/TODO.md S.2).
describe("the call parser", () => {
  test("every method parses from both forms to the same call", () => {
    const same = (text: string, json: string) => expect(parseCall(text)).toEqual(parseCall(json));
    same("help", '{"method":"help"}');
    same("whoami", '{"method":"whoami"}');
    same("post #dev hello world", '{"method":"post","channel":"#dev","text":"hello world"}');
    same("read #dev", '{"method":"read","channel":"#dev"}');
    same("read #dev 50", '{"method":"read","channel":"#dev","limit":50}');
    same("sub list", '{"method":"sub.list"}');
    same("sub add #dev #worker ^agent:", '{"method":"sub.add","channel":"#dev","target":"#worker","filter":"^agent:"}');
    same("sub add C123 #worker", '{"method":"sub.add","channel":"C123","target":"#worker"}');
    same("sub edit abc123 target=#other", '{"method":"sub.edit","id":"abc123","target":"#other"}');
    same("sub delete abc123", '{"method":"sub.delete","id":"abc123"}');
    same("channels", '{"method":"channels"}');
    same("channels prod", '{"method":"channels","filter":"prod"}');
    same("users", '{"method":"users"}');
    same("user someone@example.com", '{"method":"user","who":"someone@example.com"}');
    same("channel #dev", '{"method":"channel","channel":"#dev"}');
    same("thread #dev 1234.56", '{"method":"thread","channel":"#dev","ts":"1234.56"}');
    same("react #dev 1234.56 :white_check_mark:", '{"method":"react","channel":"#dev","ts":"1234.56","emoji":":white_check_mark:"}');
    same("edit #dev 1234.56 new text here", '{"method":"edit","channel":"#dev","ts":"1234.56","text":"new text here"}');
    same("delete #dev 1234.56", '{"method":"delete","channel":"#dev","ts":"1234.56"}');
    same("join #dev", '{"method":"join","channel":"#dev"}');
  });

  test("a multi-word filter runs to the end of the line, in add and in edit", () => {
    expect(parseCall("sub add #dev #worker ^agent: keep going")).toEqual({ method: "sub.add", channel: "#dev", target: "#worker", filter: "^agent: keep going" });
    expect(parseCall('sub edit abc filter=^agent: with spaces')).toEqual({ method: "sub.edit", id: "abc", edits: { filter: "^agent: with spaces" } });
    expect(parseCall("sub edit abc channel=#x filter=")).toEqual({ method: "sub.edit", id: "abc", edits: { channel: "#x", filter: "" } });
  });

  test("JSON-only fields parse: thread_ts, username, icon, and numbers stay timestamps", () => {
    expect(parseCall('{"method":"post","channel":"#d","text":"t","thread_ts":"123.4","username":"bot","icon":":fire:"}'))
      .toEqual({ method: "post", channel: "#d", text: "t", thread_ts: "123.4", username: "bot", icon: ":fire:" });
    expect(parseCall('{"method":"post","channel":"#d","text":"t","thread_ts":123.4}').thread_ts).toBe("123.4");
  });

  test("unknown and malformed calls answer help", () => {
    for (const bad of [
      "", "frobnicate", "post", "post #dev", "read", "read #dev nope", "read #dev 5 6",
      "sub", "sub frob", "sub add #dev", "sub add", "sub edit", "sub edit abc", "sub edit abc frob=1",
      "sub delete", "channels a b", "user", "channel", "thread #dev", "thread #dev 1 2 3",
      "react #dev 1", "edit #dev", "delete #dev", "join", "help extra words maybe not",
      "{", '{"method":"post"}', '{"method":"post","channel":"#d","text":5}', '{"method":"unknown"}',
      '{"method":"read","channel":""}', '{"method":"sub.edit","id":"a","edits":"x"}', "[]",
    ]) {
      expect(parseCall(bad)).toEqual({ method: "help" });
    }
  });
});
