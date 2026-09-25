// Sets the ps line the way server.ts does, says so, and waits to be read.
import * as proctitle from "../proctitle.ts";
proctitle.start("agent-bus-web", "9.9.9", () => 7);
console.log("titled");
await Bun.sleep(10_000);
