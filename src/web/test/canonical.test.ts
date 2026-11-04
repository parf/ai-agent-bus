import {expect, test} from "bun:test";
import {canonicalInput} from "../canonical.ts";
import {isPersonal, recordTags} from "../tags.ts";
import {match} from "../router.ts";
import "../pages/index.ts";

test("only the personal tag selects the Personal classification", () => {
  expect(isPersonal({tags:["personal"]})).toBe(true);
  expect(isPersonal({tags:[]})).toBe(false);
  expect(recordTags({tags:["ops","personal"]})).toEqual(["ops","personal"]);
  expect(()=>recordTags({personal:true} as never)).toThrow("not a record field");
  expect(()=>recordTags({personal:false} as never)).toThrow("not a record field");
});
test("retired query and form fields refuse instead of changing or clearing a filter", () => {
  canonicalInput(new URLSearchParams("tag=personal"),new URLSearchParams("tags=personal"));
  for (const name of ["personal","Personal"])
    expect(()=>canonicalInput(new URLSearchParams(`${name}=1`))).toThrow("tag=personal");
  for (const name of ["personal","edit_personal","from_personal"])
    expect(()=>canonicalInput(new URLSearchParams(),new URLSearchParams(`${name}=on`))).toThrow("Use tags");
});
test("retired channel routes do not redirect or invoke a compatibility handler", () => {
  for (const path of ["/channel","/channel/edit","/channels","/channels/new"])
    expect(match("GET",path).known).toBe(false);
  for (const path of ["/queue","/queues","/pubsub","/pubsub/channel"])
    expect(match("GET",path).route).toBeDefined();
});

import {makeHandler,webArguments} from "../server.ts";
import type {Config} from "../config.ts";
test("the HTTP entry point refuses retired fields before a handler can act", async () => {
  const saved=globalThis.fetch;
  const calls:string[]=[];
  globalThis.fetch=(async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push(`${init?.method ?? "GET"} ${String(input)}`);
    return new Response(JSON.stringify({owner:"owner",host:"fixture",protocol:1,protocol_min:1}),{headers:{"content-type":"application/json"}});
  }) as typeof fetch;
  try {
    const handler=makeHandler({daemon:"http://fixture.invalid",dev:false} as Config);
    const query=await handler(new Request("http://localhost/agents?personal=1"));
    expect(query.status).toBe(400);
    const form=await handler(new Request("http://localhost/record",{method:"POST",headers:{host:"localhost",origin:"http://localhost","content-type":"application/x-www-form-urlencoded"},body:"action=create&personal=on"}));
    expect(form.status).toBe(400);
    expect(calls.every(call=>call.startsWith("GET "))).toBe(true);
  } finally {globalThis.fetch=saved;}
});

test("version is the sole web argument and cannot bypass invalid options",()=>{
  expect(webArguments([])).toBe("start");
  expect(webArguments(["--version"])).toBe("version");
  for(const args of [["-version"],["--unknown"],["--version","--unknown"],["--unknown","--version"]])
    expect(()=>webArguments(args)).toThrow("Unknown web option");
});
