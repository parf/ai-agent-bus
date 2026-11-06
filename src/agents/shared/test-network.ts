// Fixture children inherit this flag; production destinations are unchanged.
export function assertTestLoopback(address: string): void {
  if (process.env.AGENT_TEST_LOOPBACK_ONLY !== "1") return;
  const host = new URL(address).hostname;
  if (host !== "127.0.0.1" && host !== "[::1]" && host !== "localhost")
    throw new Error("test endpoint must be loopback");
}
