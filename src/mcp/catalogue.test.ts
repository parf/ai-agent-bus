import { describe, expect, test } from "bun:test";
import { catalogue } from "./catalogue.ts";

describe("catalogue reader count", () => {
  test("distinguishes unavailable from measured zero", () => {
    expect(catalogue({ name: "old@h", kind: "agent", owner: "owner@h" })).toContain("readers: unavailable");
    expect(catalogue({ name: "new@h", kind: "agent", owner: "owner@h", readers: 0 })).toContain("readers: 0 outstanding");
  });

  test("keeps external protocol and its inbox count separate", () => {
    const text = catalogue({ name: "db@h", kind: "agent", owner: "owner@h", protocol: "mysql", readers: 2 });
    expect(text).toContain("speaks mysql");
    expect(text).toContain("readers: 2 outstanding");
  });
});
