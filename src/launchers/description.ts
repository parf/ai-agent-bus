// A purpose is optional; runtime names and paths already belong to the address.
export function descriptionOption(args: string[]): { args: string[]; description: string } {
  const forwarded: string[] = [];
  let description = "", seen = false;
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--") { forwarded.push(...args.slice(i)); break; }
    if (arg !== "--descr" && !arg.startsWith("--descr=")) { forwarded.push(arg); continue; }
    if (seen) throw new Error("give --descr only once");
    seen = true;
    const value = arg === "--descr" ? args[++i] : arg.slice("--descr=".length);
    if (value === undefined || (arg === "--descr" && value.startsWith("--"))) throw new Error("--descr needs a short purpose");
    description = value.trim();
  }
  return { args: forwarded, description };
}
