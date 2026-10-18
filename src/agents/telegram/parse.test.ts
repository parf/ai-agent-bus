import { expect, test } from "bun:test";
import { help } from "./help.ts";
import { isChat, maxLimit, methods, parseCall } from "./parse.ts";

// TG.2 (Plans/R1.0-Release/telegram/TODO.md): the text form and the JSON
// form parse to one call; the optional chat follows the README's first-word
// rule; anything unknown or malformed is help.

const case_ = (body: string, json: string, want: any) => {
  expect(parseCall(body)).toEqual(want);
  expect(parseCall(json)).toEqual(want);
};

test("every method parses from both forms to the same call", () => {
  case_("post #dev hello there", `{"method":"post","channel":"#dev","text":"hello there"}`, { method: "post", channel: "#dev", text: "hello there" });
  case_("post hello there", `{"method":"post","text":"hello there"}`, { method: "post", text: "hello there" });
  case_("read", `{"method":"read"}`, { method: "read", limit: 20 });
  case_("read #dev 5", `{"method":"read","channel":"#dev","limit":5}`, { method: "read", channel: "#dev", limit: 5 });
  case_("channels", `{"method":"channels"}`, { method: "channels" });
  case_("channels dev", `{"method":"channels","filter":"dev"}`, { method: "channels", filter: "dev" });
  case_("channel #dev", `{"method":"channel","channel":"#dev"}`, { method: "channel", channel: "#dev" });
  case_("users", `{"method":"users"}`, { method: "users" });
  case_("users alice", `{"method":"users","filter":"alice"}`, { method: "users", filter: "alice" });
  case_("user @alice", `{"method":"user","who":"@alice"}`, { method: "user", who: "@alice" });
  case_("react #dev 42 👍", `{"method":"react","channel":"#dev","id":42,"emoji":"👍"}`, { method: "react", channel: "#dev", id: "42", emoji: "👍" });
  case_("edit #dev 42 new text", `{"method":"edit","channel":"#dev","id":42,"text":"new text"}`, { method: "edit", channel: "#dev", id: "42", text: "new text" });
  case_("delete #dev 42", `{"method":"delete","channel":"#dev","id":42}`, { method: "delete", channel: "#dev", id: "42" });
  case_("sub add #dev #target relay", `{"method":"sub.add","channel":"#dev","target":"#target","filter":"relay"}`, { method: "sub.add", channel: "#dev", target: "#target", filter: "relay" });
  case_("sub add #dev #target", `{"method":"sub.add","channel":"#dev","target":"#target"}`, { method: "sub.add", channel: "#dev", target: "#target" });
  case_("sub list", `{"method":"sub.list"}`, { method: "sub.list" });
  case_("sub edit 3 channel=#new", `{"method":"sub.edit","id":"3","channel":"#new"}`, { method: "sub.edit", id: "3", edits: { channel: "#new" } });
  case_("sub edit 3 target=#new filter=hi", `{"method":"sub.edit","id":"3","target":"#new","filter":"hi"}`, { method: "sub.edit", id: "3", edits: { target: "#new", filter: "hi" } });
  case_("sub delete 3", `{"method":"sub.delete","id":"3"}`, { method: "sub.delete", id: "3" });
  case_("whoami", `{"method":"whoami"}`, { method: "whoami" });
  case_("help", `{"method":"help"}`, { method: "help" });
});

test("the first word of post and read is a chat only when it names one", () => {
  // #alias, @alias and integers are chats; a bare word is the text's start.
  expect(parseCall("post 12345 hi")).toEqual({ method: "post", channel: "12345", text: "hi" });
  expect(parseCall("post -1001234567890 hi")).toEqual({ method: "post", channel: "-1001234567890", text: "hi" });
  expect(parseCall("post @alice hi")).toEqual({ method: "post", channel: "@alice", text: "hi" });
  expect(parseCall("post hello")).toEqual({ method: "post", text: "hello" });
  expect(parseCall("post hello world")).toEqual({ method: "post", text: "hello world" });
  expect(isChat("#dev") && isChat("@dev") && isChat("-5") && isChat("5")).toBe(true);
  expect(isChat("dev") || isChat("5x") || isChat("")).toBe(false);
  // A bare `read` reads the default; `read hello` is malformed, not a chat.
  expect(parseCall("read")).toEqual({ method: "read", limit: 20 });
  expect(parseCall("read hello")).toEqual({ method: "help" });
  expect(parseCall("read hello 5")).toEqual({ method: "help" });
  // The tail keeps the original bytes: spaces, tabs and newlines.
  expect(parseCall("post #dev line1\n  line2\t")).toEqual({ method: "post", channel: "#dev", text: "line1\n  line2" });
  // The JSON chat must name a chat too: a bare word is no call at all.
  expect(parseCall(`{"method":"post","channel":"hello","text":"x"}`)).toEqual({ method: "help" });
});

test("limits are bounded and typed", () => {
  expect(parseCall("read #dev 200").limit).toBe(200);
  expect(maxLimit).toBe(200);
  expect(parseCall("read #dev 0")).toEqual({ method: "help" });
  expect(parseCall("read #dev 201")).toEqual({ method: "help" });
  expect(parseCall(`{"method":"read","channel":"#dev","limit":0}`)).toEqual({ method: "help" });
  expect(parseCall(`{"method":"read","limit":"five"}`)).toEqual({ method: "help" });
});

test("unknown or malformed is help — never a guess", () => {
  for (const body of [
    "", "  ", "--help", "help me", "POST #dev hi", "nonsense", "post", "read a b c d",
    "not json {", `{}`, `{"method":42}`, `[]`, `{"method":"post","text":""}`,
    `{"method":"read","channel":"#dev","limit":1e9}`, `{"method":"wat"}`,
    `{"method":"sub.edit","id":"3"}`, // nothing named is not an edit
    `{"method":"sub.edit","id":"3","wat":"x"}`,
  ]) {
    expect(parseCall(body)).toEqual({ method: "help" });
  }
  expect(parseCall("post")).toEqual({ method: "help" }); // no text at all
  // --help is the ask, and help answers it too.
  expect(parseCall("--help")).toEqual({ method: "help" });
});

test("help and the parser list the same set (tg1-review P3-2)", () => {
  const samples: Record<string, string> = {
    "post": "post #d hi", "read": "read #d", "channels": "channels", "channel": "channel #d",
    "users": "users", "user": "user @a", "react": "react #d 5 👍", "edit": "edit #d 5 hi",
    "delete": "delete #d 5", "sub add": "sub add #d #t", "sub list": "sub list",
    "sub edit": "sub edit 1 channel=#d", "sub delete": "sub delete 1", "whoami": "whoami", "help": "help",
  };
  // The parser accepts a sample for every method it knows.
  for (const m of methods) {
    const parsed = parseCall(samples[m]!);
    expect(parsed.method.replace(".", " ")).toBe(m);
  }
  // Every indented help line names a method, and nothing extra: both ways.
  const listed = [...help.matchAll(/^  ([a-z]+(?: [a-z]+)?)/gm)].map(m => m[1]!);
  const withoutHelp = methods.filter(m => m !== "help");
  for (const line of listed) expect(withoutHelp).toContain(line);
  expect(listed.length).toBe(withoutHelp.length);
});
