import { existsSync, readFileSync, readdirSync, readlinkSync } from "node:fs";
import { spawnSync } from "node:child_process";

export class ForeignListener extends Error {
  constructor(port: number) { super(`port ${port} is held by another process; not sending credentials`); }
}

// Inspect the kernel's listener, not a response from an untrusted HTTP peer.
// No listener yet is normal during startup; an unidentifiable owner is not.
export function ownedListener(port: number, child: number): boolean {
  const owners = new Set<number>();
  const parents = new Map<number, number>();
  if (existsSync("/proc/net/tcp")) {
    const inodes = new Set<string>();
    for (const file of ["/proc/net/tcp", "/proc/net/tcp6"]) {
      if (!existsSync(file)) continue;
      for (const line of readFileSync(file, "utf8").trim().split("\n").slice(1)) {
        const fields = line.trim().split(/\s+/);
        if (fields[3] === "0A" && parseInt(fields[1]!.split(":")[1]!, 16) === port) inodes.add(fields[9]!);
      }
    }
    if (!inodes.size) return false;
    const found = new Set<string>();
    for (const pid of readdirSync("/proc").filter(p => /^\d+$/.test(p))) {
      try {
        const stat = readFileSync(`/proc/${pid}/stat`, "utf8").split(") ").at(-1)!.split(" ");
        parents.set(Number(pid), Number(stat[1]));
        for (const fd of readdirSync(`/proc/${pid}/fd`)) {
          try {
            const inode = /^socket:\[(\d+)\]$/.exec(readlinkSync(`/proc/${pid}/fd/${fd}`))?.[1];
            if (inode && inodes.has(inode)) { found.add(inode); owners.add(Number(pid)); }
          } catch { /* descriptor closed during inspection */ }
        }
      } catch { /* another account, or a process that has exited */ }
    }
    if (found.size !== inodes.size) throw new ForeignListener(port);
  } else {
    const listing = spawnSync("lsof", ["-nP", `-iTCP:${port}`, "-sTCP:LISTEN", "-t"], { encoding: "utf8", timeout: 1000 });
    if (listing.status === 1 && !listing.stdout.trim() && !listing.stderr.trim()) return false;
    if (listing.status !== 0) throw new ForeignListener(port);
    for (const pid of listing.stdout.trim().split(/\s+/)) owners.add(Number(pid));
    const ps = spawnSync("ps", ["-axo", "pid=,ppid="], { encoding: "utf8", timeout: 1000 });
    if (ps.status !== 0) throw new ForeignListener(port);
    for (const row of ps.stdout.trim().split("\n")) {
      const [pid, parent] = row.trim().split(/\s+/).map(Number);
      parents.set(pid!, parent!);
    }
  }
  for (let owner of owners) {
    const seen = new Set<number>();
    while (owner !== child && owner > 1 && !seen.has(owner)) { seen.add(owner); owner = parents.get(owner) ?? 0; }
    if (owner !== child) throw new ForeignListener(port);
  }
  if (!owners.size) throw new ForeignListener(port);
  return true;
}

// The caller cannot probe with credentials until this returns. A collision
// gets one fresh port; cleanup is limited to the child we started.
export async function startOwnedServer<T extends { proc: { pid?: number; exitCode: number | null } }>(
  start: (port: number) => T, stop: (child: T) => Promise<void>,
): Promise<{ child: T; port: number }> {
  for (let attempt = 0; attempt < 2; attempt++) {
    const reservation = Bun.listen({ hostname: "127.0.0.1", port: 0, socket: { data() {} } });
    const port = reservation.port;
    reservation.stop(true);
    const child = start(port);
    try {
      for (let i = 0; i < 200; i++) {
        // Inspect before exitCode: a collision may already have made bind fail.
        if (child.proc.pid && ownedListener(port, child.proc.pid)) return { child, port };
        if (child.proc.exitCode !== null) throw new Error("runtime server exited before owning its listener");
        await Bun.sleep(50);
      }
      throw new Error(`runtime server did not own port ${port} before startup timed out`);
    } catch (error) {
      await stop(child);
      if (!(error instanceof ForeignListener) || attempt === 1) throw error;
    }
  }
  throw new Error("runtime server startup exhausted");
}
