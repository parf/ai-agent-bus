import { Bus } from "./bus.ts";

export function withFixtureSharing(allow: string[] | undefined): string[] {
  return [...new Set([...(allow ?? []), "*"])];
}

// Message-flow fixtures opt into peer delivery. Production registration keeps
// its restrictive default; these tests exercise transport, not that default.
export async function shareFixtureInbox(bus: Bus): Promise<void> {
  const record = (await bus.ls()).find(r => r.name === bus.name);
  if (!record) throw new Error(`fixture inbox ${bus.name} was not registered`);
  await bus.register({ ...record, allow: withFixtureSharing(record.allow) });
}
