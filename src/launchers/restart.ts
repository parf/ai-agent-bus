// Launcher-owned flag; never forwarded to a runtime.
export function restartOption(args: string[]): { args: string[]; delay?: number } {
  let delay: number | undefined;
  const forwarded = args.filter(arg => {
    if (arg !== "--restart" && !arg.startsWith("--restart=")) return true;
    if (delay !== undefined) throw new Error("give --restart only once");
    const value = arg === "--restart" ? "30" : arg.slice("--restart=".length);
    if (!/^[1-9][0-9]*m?$/.test(value)) throw new Error("--restart expects positive seconds or minutes, e.g. --restart=10 or --restart=2m");
    delay = Number(value.replace(/m$/, "")) * (value.endsWith("m") ? 60000 : 1000);
    if (!Number.isSafeInteger(delay) || delay > 2147483647) throw new Error("--restart timeout is too large");
    return false;
  });
  return { args: forwarded, delay };
}
