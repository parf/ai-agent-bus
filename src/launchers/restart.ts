// Launcher-owned flag; never forwarded to a runtime.
export function restartOption(args: string[]): { args: string[]; delay?: number } {
  let delay: number | undefined;
  const forwarded: string[] = [];
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--") { forwarded.push(...args.slice(i)); break; }
    if (arg !== "--restart" && !arg.startsWith("--restart=")) { forwarded.push(arg); continue; }
    if (delay !== undefined) throw new Error("give --restart only once");
    const value = arg === "--restart" ? "30" : arg.slice("--restart=".length);
    if (!/^[1-9][0-9]*m?$/.test(value)) throw new Error("--restart expects positive seconds or minutes, e.g. --restart=10 or --restart=2m");
    delay = Number(value.replace(/m$/, "")) * (value.endsWith("m") ? 60000 : 1000);
    if (!Number.isSafeInteger(delay) || delay > 2147483647) throw new Error("--restart timeout is too large");
  }
  return { args: forwarded, delay };
}

// Count short-lived foreground failures across the waits. A stable TUI resets
// the sequence; sidecar exits and clean quits never enter this policy.
export class RestartBackoff {
  private failures = 0;
  constructor(private readonly base: number, private readonly clock = () => performance.now()) {}
  started(): number { return this.clock(); }
  failed(started: number): { delay: number; attempt: number } {
    if (this.clock() - started >= 60_000) this.failures = 0;
    this.failures++;
    if (this.failures >= 10) throw new Error("foreground runtime failed 10 consecutive times before staying alive for 60s; stopping restart loop");
    return {delay: Math.min(300_000, this.base * 2 ** (this.failures - 1)), attempt: this.failures};
  }
}
