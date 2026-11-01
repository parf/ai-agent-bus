import { expect, test } from "bun:test";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import descriptions from "./tools.yaml";
import { allowedTools, CLAUDE_TOOL_PREFIX, codexPrompted, FILE_TOOLS } from "./tool-access.ts";

// The tools the face serves, read from the face itself: what a runtime sees.
async function served(): Promise<{ name: string; inputSchema: { properties?: Record<string, unknown> } }[]> {
  const daemon = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) => {
    const path = new URL(req.url).pathname;
    if (path === "/identity") return Response.json({ protocol: 1, protocol_min: 1 });
    if (path === "/ls") return Response.json([]);
    if (path === "/register") return Response.json({ name: "#face@test", kind: "agent", owner: "o" });
    return new Response("no", { status: 404 });
  } });
  const client = new Client({ name: "access-test", version: "1" });
  await client.connect(new StdioClientTransport({
    command: process.execPath, args: [`${import.meta.dir}/server.ts`], stderr: "pipe",
    env: { PATH: process.env.PATH ?? "", AGENT_BUS_ADDR: `http://127.0.0.1:${daemon.port}`, AGENT_BUS_TOKEN: "t", AGENT_BUS_NAME: "#face@test" },
  }));
  try { return (await client.listTools()).tools as any; } finally { await client.close(); daemon.stop(true); }
}

// Any property that names a place on disk marks a tool as touching the filesystem.
const placeLike = /(path|file|dir|folder)/i;

test("every served tool whose schema takes a path is listed as file-touching", async () => {
  const tools = await served();
  const touching = tools.filter((t) => Object.keys(t.inputSchema.properties ?? {}).some((p) => placeLike.test(p))).map((t) => t.name);
  expect(touching.length).toBeGreaterThan(0);
  const unlisted = touching.filter((n) => !(n in FILE_TOOLS));
  expect(unlisted, `file-touching tools missing from FILE_TOOLS in tool-access.ts: ${unlisted}`).toEqual([]);
  // And the list names no tool that is gone or does not take a path.
  expect(Object.keys(FILE_TOOLS).sort()).toEqual(touching.sort());
});

test("the pre-approved Claude tools are every safe tool by name and no file-touching one", async () => {
  const served_ = (await served()).map((t) => t.name).sort();
  expect(Object.keys(descriptions).sort()).toEqual(served_);
  const allowed = allowedTools(Object.keys(descriptions));
  for (const name of served_) {
    const listed = allowed.includes(CLAUDE_TOOL_PREFIX + name);
    expect(listed, `${name} ${name in FILE_TOOLS ? "touches files and must not be pre-approved" : "is a safe tool and must be pre-approved"}`).toBe(!(name in FILE_TOOLS));
  }
  expect(allowed.some((a) => a.includes("*"))).toBe(false);
});

test("Codex asks for every file-touching tool and for no other", () => {
  const parsed = Bun.TOML.parse(`x={${codexPrompted()}}`) as any;
  const asking = Object.entries(parsed.x.tools).filter(([, v]: any) => v.approval_mode === "prompt").map(([k]) => k).sort();
  expect(asking).toEqual(Object.keys(FILE_TOOLS).sort());
});
