// Deterministic model decisions only. The native runtime must execute every
// tool and return its real output; this server never calls MCP or the bus.
import { appendFileSync } from "node:fs";
import { join } from "node:path";

export class RuntimeModelFixture {
  readonly requests: any[] = [];
  held = false;
  released = false;
  listed = false;
  denied = false;
  serviceSent = false;
  replied = false;
  sawServiceResponse = "";
  #sequence = 0;
  #release!: () => void;
  #hold = new Promise<void>(resolve => { this.#release = resolve; });
  constructor(readonly slot: number, readonly dir: string, readonly secretPrefix: string, readonly runtime: "codex" | "opencode") {}
  release() { this.released = true; this.#release(); }
  async respond(req: Request): Promise<Response> {
    const body = await req.json() as any;
    this.requests.push(body);
    appendFileSync(join(this.dir, "provider.jsonl"), JSON.stringify(body) + "\n");
    const input = this.runtime === "codex" ? body.input ?? [] : (body.messages ?? []).map((m: any) => m.role === "tool" ? { type: "function_call_output", call_id: m.tool_call_id, output: m.content } : m);
    const output = (name: string) => input.findLast((x: any) => x.type === "function_call_output" && x.call_id === `${name}-${this.slot}`);
    const user = input.filter((x: any) => x.role === "user").map((x: any) => JSON.stringify(x)).join("\n");
    let item: any;
    const message = (text: string) => ({ type: "message", id: `msg-${++this.#sequence}`, role: "assistant", status: "completed", content: [{ type: "output_text", text, annotations: [] }] });
    const call = (id: string, name: string, args: unknown) => ({ type: "function_call", id: `fc-${++this.#sequence}`, call_id: `${id}-${this.slot}`, namespace: "mcp__agent_bus", name, arguments: JSON.stringify(args), status: "completed" });
    if (this.runtime === "opencode" && body.messages?.[0]?.content?.includes("You are a title generator")) {
      item = message(`Fixture session ${this.slot}`);
    } else if (!user.includes(`KICKOFF-${this.slot}`)) {
      if (!user.includes(`KEYBOARD-${this.slot}`)) throw new Error("unexpected model input");
      item = message(`KEYBOARD-OK-${this.slot}`);
    } else {
      if (!this.released) {
        this.held = true;
        await Promise.race([this.#hold, Bun.sleep(15000).then(() => { throw new Error("mid-exchange probe did not release model"); })]);
      }
      const names = this.runtime === "codex"
        ? body.tools?.find((t: any) => t.name === "mcp__agent_bus")?.tools?.map((t: any) => t.name)
        : body.tools?.map((t: any) => t.function?.name?.replace(/^agent-bus_/, ""));
      if (!["ab_ls", "ab_send"].every(n => names?.includes(n))) throw new Error("runtime did not load the real MCP tools");
      const response = user.match(new RegExp(`${this.secretPrefix}-[a-f0-9]+`))?.[0];
      if (response) {
        this.sawServiceResponse = response;
        if (!output("reply")) item = call("reply", "ab_send", { to: `#peer-${this.slot}@fixture`, topic: "interactive", tag: `slot-${this.slot}`, text: response });
        else {
          if (!JSON.stringify(output("reply")).includes("the bus accepted")) throw new Error("reply tool did not succeed");
          this.replied = true; item = message(`COMPLETE-${this.slot}: ${response}`);
        }
      } else if (!output("list")) item = call("list", "ab_ls", {});
      else if (!output("deny")) {
        const listing = JSON.stringify(output("list"));
        if (!listing.includes("#echo@fixture") || listing.includes("#forbidden@fixture")) throw new Error("MCP listing absent or leaked hidden service");
        this.listed = true;
        item = call("deny", "ab_send", { to: "#forbidden@fixture", text: "must refuse" });
      } else if (!output("service")) {
        const denial = JSON.stringify(output("deny"));
        if (denial.includes("the bus accepted") || !/not found|not visible|refused|forbidden/i.test(denial)) throw new Error("forbidden MCP send did not refuse: " + denial);
        this.denied = true;
        item = call("service", "ab_send", { to: "#echo@fixture", topic: "interactive", tag: `slot-${this.slot}`, text: `QUESTION-${this.slot}` });
      } else {
        if (!JSON.stringify(output("service")).includes("the bus accepted")) throw new Error("service call did not reach daemon");
        this.serviceSent = true; item = message(`REQUEST-SENT-${this.slot}`);
      }
    }
    if (this.runtime === "opencode") {
      const id = `chatcmpl-${this.slot}-${this.#sequence}`;
      const chunk = (delta: unknown, finish_reason: string | null = null) => ({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model: "test", choices: [{ index: 0, delta, finish_reason }] });
      const delta = item.type === "function_call" ? { role: "assistant", tool_calls: [{ index: 0, id: item.call_id, type: "function", function: { name: "agent-bus_" + item.name, arguments: item.arguments } }] } : { role: "assistant", content: item.content[0].text };
      return new Response([chunk(delta), chunk({}, item.type === "function_call" ? "tool_calls" : "stop")].map(e => `data: ${JSON.stringify(e)}\n\n`).join("") + "data: [DONE]\n\n", { headers: { "content-type": "text/event-stream" } });
    }
    const response = { id: `resp-${this.slot}-${this.#sequence}`, object: "response", created_at: Math.floor(Date.now() / 1000), status: "completed", output: [item], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2 } };
    const events: any[] = [{ type: "response.created", response: { ...response, status: "in_progress", output: [] } }, { type: "response.output_item.added", output_index: 0, item: { ...item, ...(item.type === "function_call" ? { arguments: "" } : { content: [] }), status: "in_progress" } }];
    if (item.type === "function_call") events.push({ type: "response.function_call_arguments.delta", output_index: 0, item_id: item.id, delta: item.arguments });
    else events.push({ type: "response.content_part.added", output_index: 0, content_index: 0, item_id: item.id, part: { type: "output_text", text: "", annotations: [] } }, { type: "response.output_text.delta", output_index: 0, content_index: 0, item_id: item.id, delta: item.content[0].text });
    events.push({ type: "response.output_item.done", output_index: 0, item }, { type: "response.completed", response });
    return new Response(events.map((event, sequence_number) => `event: ${event.type}\ndata: ${JSON.stringify({ ...event, sequence_number })}\n\n`).join(""), { headers: { "content-type": "text/event-stream" } });
  }
}
