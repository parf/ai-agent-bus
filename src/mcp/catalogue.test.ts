import { describe, expect, test } from "bun:test";
import { ago, catalogue } from "./catalogue.ts";

describe("catalogue reader count", () => {
  test("distinguishes unavailable from measured zero", () => {
    expect(catalogue({ name: "old@h", kind: "agent", owner: "owner@h" })).toContain("readers: unavailable");
    expect(catalogue({ name: "new@h", kind: "agent", owner: "owner@h", readers: 0 })).toContain("readers: 0 outstanding");
  });

  test("keeps external protocol and its queue count separate", () => {
    // A service is the external kind, which is the only one an address and a
    // protocol belong to. See docs/03-records.md#record-kinds.
    const text = catalogue({ name: "db@h", kind: "service", owner: "owner@h", addr: "db.example:3306", protocol: "mysql", readers: 2 });
    expect(text).toContain("speaks mysql at db.example:3306");
    // It has no queue here, so it makes no claim about one.
    expect(text).not.toContain("readers");
    expect(text).not.toContain("queued");
  });
});

describe("catalogue last use", () => {
  test("says how long ago a name was last used, and nothing when it never was", () => {
    const now = Date.parse("2026-09-23T12:00:00Z");
    expect(catalogue({ name: "#a@h", kind: "agent", owner: "o", readers: 1, last_used: "2026-09-23T11:57:00Z" }, now)).toContain("last used 3m ago");
    expect(catalogue({ name: "#b@h", kind: "agent", owner: "o", readers: 0 }, now)).not.toContain("last used");
    expect(ago("2026-09-23T11:59:30Z", now)).toBe("just now");
    expect(ago("2026-09-23T07:00:00Z", now)).toBe("5h ago");
    expect(ago("2026-09-20T12:00:00Z", now)).toBe("3d ago");
  });
});
