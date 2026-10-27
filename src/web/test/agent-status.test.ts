import { expect, test } from "bun:test";
import { agentStatus, StatusAnswer } from "../agent-status.tsx";
import { render } from "../jsx.ts";
import type { Ctx } from "../ctx.ts";

test("viewed agent status is bounded, escaped, and absent when unsupported or offline", async () => {
  let body = "ok: <img src=x onerror=alert(1)>";
  let readers = 1, calls = 0;
  const ctx = {q:()=>"#bus", lookup:async()=>({name:"#bus",kind:"agent",readers}), call:async(name:string, input:string, wait:number, ask:string)=>{
    calls++; expect(name).toBe("#bus"); expect(input).toBe(""); expect(ask).toBe("status"); expect(wait).toBeLessThanOrEqual(2000);
    return {kind:"answer",body};
  }} as unknown as Ctx;
  let page = await (await agentStatus(ctx)).text();
  expect(page).toContain('class="pill tone-ok"');
  expect(page).toContain("&lt;img"); expect(page).not.toContain("<img");
  body = "unsupported: status";
  expect(await (await agentStatus(ctx)).text()).toBe("");
  readers=0; const before=calls;
  expect(await (await agentStatus(ctx)).text()).toBe(""); expect(calls).toBe(before);
  expect(render(StatusAnswer({body:"surprise: detail"}))).toContain('tone-muted');
  for(const [word,tone] of [["degraded","warn"],["failed","bad"],["starting","neutral"],["stopping","neutral"],["unknown","muted"]])
    expect(render(StatusAnswer({body:`${word}: detail`}))).toContain(`tone-${tone}`);
});
