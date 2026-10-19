import { expect, test } from "bun:test";
import { formatConfiguration } from "../configuration.tsx";

test("configuration formatting changes only JSON whitespace", () => {
  const compact = String.raw`{"huge":9007199254740993,"escape":"\u0061","same":1,"same":2,"array":[{},true,"a b"]}`;
  const pretty = formatConfiguration(compact, true);
  expect(pretty).toContain('\n  "huge": 9007199254740993,');
  expect(formatConfiguration(pretty)).toBe(compact);
  expect(formatConfiguration('{\n "a": 1\n}')).toBe('{"a":1}');
});

import { configuration } from "../configuration.tsx";
import type { Ctx, Rec } from "../ctx.ts";
test("configuration never fetches or shows a non-manager's value", async () => {
  let reads = 0;
  const ctx = { bus: async () => { reads++; return '{"channel":"protected"}'; } } as unknown as Ctx;
  expect(await configuration(ctx, {name:"#worker@h", kind:"agent", can_manage:false} as Rec)).toBeUndefined();
  expect(reads).toBe(0);
  expect(await configuration(ctx, {name:"#worker@h", kind:"agent", can_manage:true} as Rec)).toContain('"channel": "protected"');
  expect(reads).toBe(1);
});

import { storeConfiguration } from "../pages/records.tsx";
test("web saves compact configuration bytes before calling the daemon", async () => {
  let wire = "";
  const ctx = {
    f: () => '{\n  "n": 9007199254740993\n}',
    bus: async (_method: string, _path: string, options: {bodyJSON:string}) => {wire = options.bodyJSON;},
  } as unknown as Ctx;
  await storeConfiguration(ctx, "#worker@h", "Saved");
  expect(wire).toBe('{"Name":"#worker@h","Config":{"n":9007199254740993}}');
});
