import { describe, expect, test } from "bun:test";
import { parseCall } from "./parse.ts";

// S.2: the text and JSON forms parse to the same call, and anything unknown
// or malformed answers help (Plans/R1.2/done/slack/TODO.md S.2).
describe("the call parser", () => {
  // Each row: the text form, the JSON form, and the call both must parse to
  // — compared to the expected call, so a parser answering help to every
  // input fails here rather than agreeing with itself (slack-review-S.2).
  const rows: [string, string, object][] = [
    ["help", '{"method":"help"}', { method: "help" }],
    ["whoami", '{"method":"whoami"}', { method: "whoami" }],
    ["post #dev hello world", '{"method":"post","channel":"#dev","text":"hello world"}', { method: "post", channel: "#dev", text: "hello world" }],
    ["read #dev", '{"method":"read","channel":"#dev"}', { method: "read", channel: "#dev", limit: 20 }],
    ["read #dev 50", '{"method":"read","channel":"#dev","limit":50}', { method: "read", channel: "#dev", limit: 50 }],
    ["sub list", '{"method":"sub.list"}', { method: "sub.list" }],
    ["sub add #dev #worker ^agent:", '{"method":"sub.add","channel":"#dev","target":"#worker","filter":"^agent:"}', { method: "sub.add", channel: "#dev", target: "#worker", filter: "^agent:" }],
    ["sub add C123 #worker", '{"method":"sub.add","channel":"C123","target":"#worker"}', { method: "sub.add", channel: "C123", target: "#worker" }],
    ["sub delete abc123", '{"method":"sub.delete","id":"abc123"}', { method: "sub.delete", id: "abc123" }],
    ["channels", '{"method":"channels"}', { method: "channels" }],
    ["channels prod", '{"method":"channels","filter":"prod"}', { method: "channels", filter: "prod" }],
    ["users", '{"method":"users"}', { method: "users" }],
    ["user someone@example.com", '{"method":"user","who":"someone@example.com"}', { method: "user", who: "someone@example.com" }],
    ["channel #dev", '{"method":"channel","channel":"#dev"}', { method: "channel", channel: "#dev" }],
    ["thread #dev 1234.56", '{"method":"thread","channel":"#dev","ts":"1234.56"}', { method: "thread", channel: "#dev", ts: "1234.56", limit: 20 }],
    ["react #dev 1234.56 :white_check_mark:", '{"method":"react","channel":"#dev","ts":"1234.56","emoji":":white_check_mark:"}', { method: "react", channel: "#dev", ts: "1234.56", emoji: ":white_check_mark:" }],
    ["edit #dev 1234.56 new text here", '{"method":"edit","channel":"#dev","ts":"1234.56","text":"new text here"}', { method: "edit", channel: "#dev", ts: "1234.56", text: "new text here" }],
    ["delete #dev 1234.56", '{"method":"delete","channel":"#dev","ts":"1234.56"}', { method: "delete", channel: "#dev", ts: "1234.56" }],
    ["join #dev", '{"method":"join","channel":"#dev"}', { method: "join", channel: "#dev" }],
  ];
  test("every method parses from both forms to its expected call", () => {
    for (const [text, json, want] of rows) {
      expect(parseCall(text)).toEqual(want);
      expect(parseCall(json)).toEqual(want);
    }
  });

  test("a tail keeps its bytes: double spaces, tabs and newlines in text and filters (slack-review-S.2)", () => {
    expect(parseCall("post C1 a  b\tc\n```\nx = 1\n```")).toEqual({ method: "post", channel: "C1", text: "a  b\tc\n```\nx = 1\n```" });
    expect(parseCall("edit C1 1.0 first\nsecond")).toEqual({ method: "edit", channel: "C1", ts: "1.0", text: "first\nsecond" });
    expect(parseCall("sub add C1 #w ^a  b$")).toEqual({ method: "sub.add", channel: "C1", target: "#w", filter: "^a  b$" });
    expect(parseCall("post\tC1   spaced  out")).toEqual({ method: "post", channel: "C1", text: "spaced  out" });
  });

  test("a JSON key the method does not take is refused by name, never ignored", () => {
    expect(parseCall('{"method":"post","chanel":"C1","text":"hi"}')).toEqual({ method: "help", why: 'post takes no "chanel"' });
    expect(parseCall('{"method":"whoami","channel":"C1"}')).toEqual({ method: "help", why: 'whoami takes no "channel"' });
  });

  test("a JSON field that is present but invalid is no call; an absent one takes its default (slack-review-S.2)", () => {
    for (const bad of [
      '{"method":"read","channel":"C1","limit":"bad"}',
      '{"method":"read","channel":"C1","limit":0}',
      '{"method":"read","channel":"C1","limit":201}',
      '{"method":"read","channel":"C1","limit":1e20}',
      '{"method":"sub.add","channel":"C1","target":"#w","filter":false}',
      '{"method":"thread","channel":"C1","ts":""}',
      '{"method":"post","channel":"C1","text":""}',
      '{"method":"channels","member_only":"yes"}',
      '{"method":"post","channel":"C1","text":"t","username":7}',
    ]) expect(parseCall(bad)).toEqual({ method: "help" });
    expect(parseCall('{"method":"read","channel":"C1","limit":200}')).toEqual({ method: "read", channel: "C1", limit: 200 });
    expect(parseCall("read C1 201")).toEqual({ method: "help" });
    expect(parseCall('{"method":"channels","member_only":true}')).toEqual({ method: "channels", member_only: true });
    expect(parseCall('{"method":"channels","member_only":false}')).toEqual({ method: "channels", member_only: false });
  });

  test("a multi-word filter runs to the end of the add line", () => {
    expect(parseCall("sub add #dev #worker ^agent: keep going")).toEqual({ method: "sub.add", channel: "#dev", target: "#worker", filter: "^agent: keep going" });
  });

  test("JSON-only fields parse: thread_ts, username, icon; a numeric timestamp is refused", () => {
    expect(parseCall('{"method":"post","channel":"#d","text":"t","thread_ts":"123.4","username":"bot","icon":":fire:"}'))
      .toEqual({ method: "post", channel: "#d", text: "t", thread_ts: "123.4", username: "bot", icon: ":fire:" });
    // A number is no timestamp: 1.10 would arrive as 1.1.
    expect(parseCall('{"method":"post","channel":"#d","text":"t","thread_ts":123.4}')).toEqual({ method: "help" });
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
      expect(parseCall(bad).method).toBe("help");
    }
  });
});

test("subscription authors have equivalent text and JSON forms, without local edits", () => {
  expect(parseCall('sub add #alerts #oncall@h from=@ann,U123,bob@x.com /error/i')).toEqual({method:'sub.add',channel:'#alerts',target:'#oncall@h',from_users:['@ann','U123','bob@x.com'],filter:'/error/i'});
  expect(parseCall(JSON.stringify({method:'sub.add',channel:'#alerts',target:'#oncall@h',from_users:['@ann','U123','bob@x.com'],filter:'/error/i'}))).toEqual(parseCall('sub add #alerts #oncall@h from=@ann,U123,bob@x.com /error/i'));
  for (const from_users of [null, 'U1', [1], [''], ['U1,U2']]) expect(parseCall(JSON.stringify({method:'sub.add',channel:'C1',target:'#t@h',from_users})).method).toBe('help');
});


test("removed sub.edit refuses both forms and names delete/add", () => {
  for (const body of ['sub edit abc target=#other', '{"method":"sub.edit","id":"abc","target":"#other"}']) {
    const result = parseCall(body);
    expect(result.method).toBe("help");
    expect((result as { why?: string }).why).toContain("sub delete");
    expect((result as { why?: string }).why).toContain("sub add");
  }
});
