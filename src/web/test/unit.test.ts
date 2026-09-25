import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { h, raw, render } from "../jsx.ts";
import { local, returnTo, sameOrigin } from "../http.ts";
import { ENTITY, DAEMON_OWNER, MAINTAINER, authority } from "../glyphs.ts";
import { STYLESHEETS, SCRIPTS, FONTS, csp } from "../assets.ts";
import { attentionItems, exchanges, type Envelope } from "../pages/node.tsx";
import { usedBy } from "../pages/people.tsx";
import { lineRefusal } from "../problem.tsx";
import { Ribbon } from "../ui/charts.tsx";
import { parseRange, bucket, addDays, KEEP_DAYS } from "../ui/range.tsx";
import { duration, relative } from "../format.ts";
import type { Rec, Status } from "../ctx.ts";
import { titleArea, titleBytes } from "../proctitle.ts";

describe("jsx", () => {
  test("text and attributes are escaped", () => {
    expect(render(h("p", { title: `"><script>` }, "<b>&"))).toBe(`<p title="&quot;&gt;&lt;script&gt;">&lt;b&gt;&amp;</p>`);
  });
  test("markup passes only as raw()", () => {
    expect(render(h("div", null, raw("<i>x</i>")))).toBe("<div><i>x</i></div>");
    expect(render(h("div", null, "<i>x</i>"))).toBe("<div>&lt;i&gt;x&lt;/i&gt;</div>");
  });
  test("an inline style or handler never renders", () => {
    expect(() => h("div", { style: "color:red" })).toThrow();
    expect(() => h("div", { onClick: "x()" })).toThrow();
  });
  test("false and null attributes drop; true is bare", () => {
    expect(render(h("input", { disabled: true, checked: false, value: null }))).toBe("<input disabled>");
  });
});

describe("return addresses", () => {
  test("local keeps only a path on this host", () => {
    expect(local("/agents?q=x")).toBe("/agents?q=x");
    for (const bad of ["//evil.example/x", "https://evil.example/", "/\\evil", "javascript:alert(1)", "", "agents"]) expect(local(bad)).toBe("/");
  });
  test("returnTo keeps only its allowed listing, without fragment", () => {
    expect(returnTo("/queues?page=2#x", ["/queues"], "/queues")).toBe("/queues?page=2");
    expect(returnTo("/users", ["/queues"], "/queues")).toBe("/queues");
    expect(returnTo("//evil/queues", ["/queues"], "/queues")).toBe("/queues");
  });
  test("sameOrigin needs a matching Origin and a same-origin fetch", () => {
    const req = (o: Record<string, string>) => new Request("http://127.0.0.1:6781/service", { method: "POST", headers: { host: "127.0.0.1:6781", ...o } });
    expect(sameOrigin(req({ origin: "http://127.0.0.1:6781" }), false)).toBe(true);
    expect(sameOrigin(req({}), false)).toBe(false);
    expect(sameOrigin(req({ origin: "http://evil.example" }), false)).toBe(false);
    expect(sameOrigin(req({ origin: "https://127.0.0.1:6781" }), false)).toBe(false);
    expect(sameOrigin(req({ origin: "http://127.0.0.1:6781", "sec-fetch-site": "cross-site" }), false)).toBe(false);
  });
});

describe("glyphs", () => {
  test("the web table matches src/internal/display", () => {
    const go = readFileSync(join(import.meta.dir, "../../internal/display/entity.go"), "utf8");
    const glyph = (kind: string) => new RegExp(`case "${kind}"(?:, "person")?:\\s*return "([^"]+)"`).exec(go.slice(go.indexOf("func EntityGlyph")))?.[1];
    for (const [k, e] of Object.entries(ENTITY)) {
      const g = k === "group" ? /GroupGlyph\s*=\s*"([^"]+)"/.exec(go)![1] : glyph(k);
      expect(`${k} ${e.glyph}`).toBe(`${k} ${g}`);
      const word = new RegExp(`case "${k}"(?:, "person")?:\\s*return (?:"[^"]*"|GroupGlyph) \\+? ?"? ?([A-Za-z]+)`).exec(go.slice(go.indexOf("func Entity(")));
      if (word) expect(e.word).toBe(word[1]!);
    }
    expect(DAEMON_OWNER.glyph).toBe(/DaemonOwnerGlyph\s*=\s*"([^"]+)"/.exec(go)![1]!);
    expect(MAINTAINER.glyph).toBe(/MaintainerGlyph\s*=\s*"([^"]+)"/.exec(go)![1]!);
    expect(authority(true, true)).toBe("🔱 Daemon owner");
    expect(authority(false, true)).toBe("Daemon administrator");
    expect(authority(false, false)).toBe("User");
  });
});

describe("external assets", () => {
  const all = [...STYLESHEETS, ...SCRIPTS];
  test("every external file is pinned to an exact version and hashed", () => {
    for (const a of all) {
      expect(a.url).toMatch(/^https:\/\/cdn\.jsdelivr\.net\/npm\/(@[^/]+\/)?[^/@]+@\d+\.\d+\.\d+\//);
      expect(a.url).not.toMatch(/\+esm|@latest/);
      expect(a.integrity).toMatch(/^sha384-[A-Za-z0-9+/]{64}$/);
    }
  });
  test("the CSP names exact files, never a bare host, and no inline style", () => {
    const p = csp();
    for (const a of all) expect(p).toContain(a.url);
    for (const d of p.split("; ")) expect(d.split(" ")).not.toContain("https://cdn.jsdelivr.net");
    expect(p).not.toContain("unsafe-inline");
    expect(p).toContain(`font-src ${FONTS.geist.files} ${FONTS.mono.files}`);
    expect(p).toContain("connect-src 'self'");
  });
});

const st = (o: Partial<Status> = {}): Status => ({ you: "me", ...o });
const rec = (o: Partial<Rec>): Rec => ({ name: "x", kind: "queue", owner: "me", ...o });

describe("attention items", () => {
  test("each condition appears once per record, in level then title order", () => {
    const items = attentionItems(st({ unclean: true, refused: { acl: 2, full: 0 }, owner_inactive: { records: 1, messages: 3 } }), [
      rec({ name: "held", status: "inactive", queued: 2, at_bound: true }),
      rec({ name: "full", at_bound: true, dropped: 1 }),
      rec({ name: "lost", expired: 1 }),
      rec({ name: "fine", queued: 9 }),
    ]);
    expect(items.map(i => `${i.level}:${i.title}:${i.key}`)).toEqual([
      "red:Queue at capacity when observed:full",
      "red:The previous stop was not clean:",
      "orange:Inactive and work is held:held",
      "orange:Messages were lost from this inbox:lost",
      "orange:Records inactive because their owner is:",
      "blue:Requests were refused:acl",
    ]);
  });
  test("no owner-inactive item without the daemon's count", () => {
    expect(attentionItems(st(), []).length).toBe(0);
  });
});

const env = (o: Partial<Envelope>): Envelope => ({ message_id: "m", from: "a", to: "b", at: "2026-09-24T01:00:00Z", ...o });

describe("exchanges", () => {
  test("a receipt on the reply route folds into its message", () => {
    const x = exchanges([env({ message_id: "m1", topic: "t", tag: "x" }), env({ message_id: "r1", from: "b", to: "a", topic: "t", tag: "x", receipt: "done", re: "m1", at: "2026-09-24T01:01:00Z" })]);
    expect(x.length).toBe(1);
    expect(x[0]!.receipts.map(r => r.message_id)).toEqual(["r1"]);
  });
  test("a receipt from a queue's consumer folds too", () => {
    const x = exchanges([env({ message_id: "m1", to: "jobs", topic: "t", tag: "x" }), env({ message_id: "r1", from: "worker", to: "a", topic: "t", tag: "x", receipt: "ack", re: "m1", at: "2026-09-24T01:01:00Z" })]);
    expect(x.length).toBe(1);
  });
  test("reply_to changes the route a receipt must travel", () => {
    const m = env({ message_id: "m1", topic: "t", tag: "x", reply_to: { name: "c" } });
    expect(exchanges([m, env({ message_id: "r1", from: "b", to: "c", topic: "t", tag: "x", receipt: "done", re: "m1", at: "2026-09-24T01:01:00Z" })]).length).toBe(1);
    const off = exchanges([m, env({ message_id: "r2", from: "b", to: "a", topic: "t", tag: "x", receipt: "done", re: "m1", at: "2026-09-24T01:01:00Z" })]);
    expect(off.length).toBe(2);
    expect(off.find(r => r.env.receipt)!.notice).toContain("route or time differs");
  });
  test("each unfoldable receipt says why", () => {
    const x = exchanges([env({ message_id: "r0", receipt: "ack" }), env({ message_id: "r1", receipt: "ack", re: "gone" })]);
    expect(x.map(r => r.notice)).toEqual(expect.arrayContaining(["Receipt has no original message reference.", "Original message not in this visible history."]));
  });
  test("a late answer is marked against the one message it answers", () => {
    const x = exchanges([env({ message_id: "m1", topic: "t", tag: "x", deadline: "2026-09-24T01:00:30Z" }), env({ message_id: "a1", from: "b", to: "a", topic: "t", tag: "x", at: "2026-09-24T01:02:00Z" })]);
    const answer = x.find(r => r.env.message_id === "a1")!;
    expect(answer.matches).toEqual(["m1"]);
    expect(answer.late).toBe(true);
  });
});

describe("used by", () => {
  test("direct, maintainer and nested uses are named", () => {
    const groups = { "@ops": ["alice"], "@all": ["@ops"] };
    const recs = [rec({ name: "q1", allow: ["@ops"] }), rec({ name: "q2", maintainers: ["@ops"] }), rec({ name: "q3", allow: ["@all"] }), rec({ name: "q4", maintainers: ["@all"] }), rec({ name: "@all", kind: "group", allow: ["@ops"] }), rec({ name: "@top", kind: "group", allow: ["@all"] })];
    expect(usedBy("@ops", recs, { ...groups, "@top": ["@all"] }).map(u => `${u.rec.name}:${u.uses.join("+")}`)).toEqual(["@all:member", "@top:member via @all", "q1:ACL", "q2:Maintainers", "q3:ACL via @all", "q4:Maintainers via @all"]);
  });
});

describe("charts", () => {
  test("a daemon value never reaches the ribbon's markup unescaped", () => {
    const html = render(Ribbon({ slots: [{ at: "</title><img src=x>", in: 1, out: 0, dropped: 0, expired: 0, refused: 0 }], label: "x" }));
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;/title&gt;");
  });
});

describe("ranges", () => {
  const ctxWith = (q: string) => ({ q: (n: string) => new URLSearchParams(q).get(n) ?? "" }) as any;
  test("a range clamps to today and to what the daemon keeps, and steps by its own length", () => {
    const today = 260924;
    const w = parseRange(ctxWith("range=week"), today);
    expect([w.from, w.to, w.prev, w.next]).toEqual([260918, 260924, 260917, undefined]);
    const past = parseRange(ctxWith("range=month&at=260801"), today);
    expect([past.from, past.to, past.prev, past.next]).toEqual([260703, 260801, 260702, 260831]);
    expect(parseRange(ctxWith("at=991231"), today).at).toBe(today);
    const oldest = addDays(today, -(KEEP_DAYS - 1));
    const first = parseRange(ctxWith("range=week&at=200101"), today);
    expect(first.from).toBe(oldest);
    expect(first.prev).toBeUndefined();
    expect(parseRange(ctxWith("range=nonsense"), today).kind).toBe("day");
    expect(parseRange(ctxWith(""), today).live).toBe(true);
    expect(parseRange(ctxWith("at=260923"), today).live).toBe(false);
  });
  test("buckets sum their slots and start at the first", () => {
    const s = Array.from({ length: 12 }, (_, i) => ({ at: `t${i}`, in: 1, out: i, dropped: 0, expired: 0, refused: 0 }));
    const b = bucket(s, 6);
    expect(b.map(x => [x.at, x.in, x.out])).toEqual([["t0", 6, 15], ["t6", 6, 51]]);
  });
});

describe("helpers", () => {
  test("line attribution finds the named line", () => {
    expect(lineRefusal("no such name: group member nosuchuser", ["alice", "nosuchuser"])).toBe(2);
    expect(lineRefusal("nothing here", ["alice"])).toBe(0);
  });
  test("durations and relative time", () => {
    expect(duration("1h2m3.5s")).toBe(3723.5);
    expect(duration("junk")).toBe(0);
    const now = new Date("2026-09-24T12:00:00Z");
    expect(relative("2026-09-24T11:58:00Z", now)).toBe("2m ago");
    expect(relative("0001-01-01T00:00:00Z", now)).toBe("—");
  });
});

describe("process title", () => {
  test("the argv bounds come after the command name, whatever it holds", () => {
    const tail = Array.from({ length: 50 }, (_, i) => String(i + 3)); // fields 3..52
    tail[45] = "1000"; tail[46] = "1050";
    expect(titleArea(`42 (b) u (n) ${tail.join(" ")}\n`)).toEqual({ start: 1000, size: 50 });
    expect(titleArea("42 (bun) S 1")).toBeNull();
  });
  test("the title is cut to the area and zero filled", () => {
    expect(Array.from(titleBytes("abcdef", 4))).toEqual([97, 98, 99, 0]);
    expect(Array.from(titleBytes("ab", 5))).toEqual([97, 98, 0, 0, 0]);
  });
  test("a running process shows the title in ps", async () => {
    const p = Bun.spawn(["bun", "run", join(import.meta.dir, "title-probe.ts"), "x".repeat(60)], { stdout: "pipe" });
    try {
      const reader = p.stdout.getReader();
      expect(new TextDecoder().decode((await reader.read()).value)).toContain("titled");
      const cmdline = readFileSync(`/proc/${p.pid}/cmdline`, "latin1").replace(/\0+$/, "");
      expect(cmdline).toBe("agent-bus-web 9.9.9 ; Calls: 7");
    } finally { p.kill(); }
  });
});
