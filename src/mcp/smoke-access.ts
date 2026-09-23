import { Bus, ownerACL } from "./bus.ts";

// A shared fixture inbox is opened to everyone. A Personal one cannot be: its
// ACL names only the owner's cohort, which `@owner` already admits, and the
// daemon refuses the wildcard there (docs/03-records.md#personal-and-shared).
export function withFixtureSharing(allow: string[] | undefined, personal = false): string[] {
  const grant = personal ? ownerACL : "*";
  return [...new Set([...(allow ?? []), grant])];
}

// Message-flow fixtures opt into peer delivery. Production registration keeps
// its restrictive default; these tests exercise transport, not that default.
export async function shareFixtureInbox(bus: Bus): Promise<void> {
  const record = (await bus.ls()).find(r => r.name === bus.name);
  if (!record) throw new Error(`fixture inbox ${bus.name} was not registered`);
  await bus.register({ ...record, allow: withFixtureSharing(record.allow, record.personal === true) });
}
