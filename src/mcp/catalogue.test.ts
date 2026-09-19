import { describe, expect, test } from "bun:test";
import { catalogue } from "./catalogue.ts";

describe("catalogue reader count", () => {
  test("distinguishes unavailable from measured zero", () => {
    expect(catalogue({ name: "old@h", kind: "agent", owner: "owner@h" })).toContain("readers: unavailable");
    expect(catalogue({ name: "new@h", kind: "agent", owner: "owner@h", readers: 0 })).toContain("readers: 0 outstanding");
  });

  test("keeps external protocol and its queue count separate", () => {
    // A service is the external kind, which is the only one an address and a
    // protocol belong to. See docs/03-services-and-topics.md#five-record-kinds.
    const text = catalogue({ name: "db@h", kind: "service", owner: "owner@h", addr: "db.example:3306", protocol: "mysql", readers: 2 });
    expect(text).toContain("speaks mysql");
    expect(text).toContain("readers: 2 outstanding");
  });
});
