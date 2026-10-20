import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { parseCall, KEYS } from "./parse.ts";
import manifest from "./agent.json";
import { helpData, secrets, helpAnswer, type Manifest } from "../shared/metadata.ts";
import { describe } from "./help.ts";

test("manifest help matches the parser and every handler", () => {
  const methods = Object.keys(manifest.methods).sort();
  expect(methods).toEqual(Object.keys(KEYS).sort());
  const code = readFileSync(new URL("./telegram.ts", import.meta.url), "utf8");
  const handlers = new Set([...code.matchAll(/case "([a-z.]+)":/g)].map(match => match[1]!));
  handlers.add("help");
  expect([...handlers].sort()).toEqual(methods);
  const examples: Record<string, unknown> = { channel: "#fixture", text: "hello", id: "1", ts: "123.456", who: "U1", emoji: "👍", target: "#target@t" };
  for (const [name, method] of Object.entries(manifest.methods)) {
    expect(method.args.map(arg => arg.name).sort()).toEqual([...KEYS[name]!].sort());
    const call: Record<string, unknown> = { method: name };
    for (const arg of method.args) if (!("optional" in arg && arg.optional)) call[arg.name] = examples[arg.name];
    if (name === "sub.edit") call.filter = "";
    expect(parseCall(JSON.stringify(call)).method).toBe(name);
  }
  const generated = JSON.parse(describe(true, "help", true));
  expect(Object.keys(generated.methods).sort()).toEqual(methods);
  expect(generated.asks).toEqual(["help", "help-config", "help-secret", "help-json"]);
  expect(manifest).not.toHaveProperty("version");
  expect(manifest.license).toBe("PolyForm-Noncommercial-1.0.0");
});

test("secret help exposes declarations and comments, never values", () => {
  const result = helpData(manifest, "help-secret", "# required key\nTOKEN=\n# optional key\nURL=https://secret-sentinel.invalid\n");
  expect(JSON.stringify(result)).not.toContain("secret-sentinel");
  expect(helpData(manifest, "help-json", "# optional\nTOKEN=secret-sentinel\n", ["owner"])).toHaveProperty("methods");
  expect(JSON.stringify(helpData(manifest, "help-json", "# optional\nTOKEN=secret-sentinel\n", ["owner"]))).not.toContain("secret-sentinel");
  expect(result).toEqual({ secrets: [{ name: "TOKEN", help: "required key", required: true }, { name: "URL", help: "optional key", required: false }] });
  const source = readFileSync(new URL("./env.dist", import.meta.url), "utf8");
  for (const key of secrets(source)) expect(key.help.length).toBeGreaterThan(0);
});


test("machine help reports only hidden counts and needed roles, with supported body formats", () => {
 const fixture: Manifest = { description:"Fixture", body_formats:["text","json"], methods:{read:{help:"Read"},secret_one:{help:"private-description",example:"private-example",role:["operate","admin"]},secret_two:{help:"private-description",role:"admin"}} };
 for (const [roles,count,needed] of [[[],2,["admin","operate"]],[["operate"],1,["admin"]],[["admin"],0,[]],[["owner"],0,[]],[["maintainer"],0,[]]] as [string[],number,string[]][]) {
  const out=JSON.parse(helpAnswer(fixture,"help-json","",roles));
  expect(out.hidden_methods).toEqual({count,roles:needed});
  expect(Object.keys(out.methods).length).toBe(3-count);
  expect(out.body_formats).toEqual(["text","json"]);
  if(count===2)for(const secret of ["private-description","private-example","secret_one","secret_two"])expect(JSON.stringify(out)).not.toContain(secret);
 }
 expect(JSON.parse(helpAnswer({description:"Legacy"},"help-json","",[])).body_formats).toEqual(["text"]);
 expect(manifest.body_formats).toEqual(["text","json"]);
 for(const [method,entry] of Object.entries(manifest.methods)){
  if(method==="help")continue;
  expect(entry.example).toContain("agent-bus call");
  const body=entry.example.match(/'([^']*)'$/)?.[1] ?? entry.example.split(/\s+/).at(-1)!;
  expect(parseCall(body).method).toBe(method);
 }
});
