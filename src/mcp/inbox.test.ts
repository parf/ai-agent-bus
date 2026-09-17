import { expect, test } from "bun:test";
import { Bus } from "./bus.ts";

test("consume forwards inbox separately from topic and tag", async () => {
  let query: URLSearchParams | undefined;
  const server = Bun.serve({
    port: 0,
    fetch(request) {
      query = new URL(request.url).searchParams;
      return new Response(null, { status: 204 });
    },
  });
  try {
    const bus = new Bus({
      AGENT_BUS_ADDR: `http://127.0.0.1:${server.port}`,
      AGENT_BUS_NAME: "reader@h",
      AGENT_BUS_TOKEN: "fixture",
    });
    await bus.consume({ inbox: "jobs@h", topic: "MyTopic", tag: "result", wait: "1ms" });
    expect(query?.get("inbox")).toBe("jobs@h");
    expect(query?.get("topic")).toBe("MyTopic");
    expect(query?.get("tag")).toBe("result");
    expect(query?.get("wait")).toBe("1ms");
  } finally {
    server.stop(true);
  }
});
