import { expect, test } from "bun:test";
import { ownerACL, withOwnerACL } from "./bus.ts";
import { withFixtureSharing } from "./smoke-access.ts";

test("launcher registration adds @owner without erasing or duplicating explicit grants", () => {
  expect(withOwnerACL(undefined)).toEqual([ownerACL]);
  expect(withOwnerACL(["peer@example", ownerACL, "@team"])).toEqual(["peer@example", ownerACL, "@team"]);
});

test("transport fixture sharing does not erase the launcher's owner cohort", () => {
  expect(withFixtureSharing([ownerACL, "peer@example"])).toEqual([ownerACL, "peer@example", "*"]);
  expect(withFixtureSharing(["*", ownerACL])).toEqual(["*", ownerACL]);
});
