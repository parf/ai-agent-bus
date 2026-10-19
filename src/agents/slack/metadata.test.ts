import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { parseCall, KEYS } from "./parse.ts";
import manifest from "./agent.json";
import { helpData, secrets } from "../shared/metadata.ts";
import { describe } from "./describe.ts";

test("manifest help matches the parser and every handler", () => {
  const methods = Object.keys(manifest.methods).sort();
  expect(methods).toEqual(Object.keys(KEYS).sort());
  const code = readFileSync(new URL("./slack.ts", import.meta.url), "utf8");
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
  const generated = JSON.parse(describe("#fixture@t", true, "help", true));
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
