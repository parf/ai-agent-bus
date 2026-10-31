// How the launcher puts a push in front of a Codex thread or an OpenCode session, and starts it again after a
// failed delivery. launcher.ts is a script, so this is the part a test can drive: the real bindThread and
// bindSession, with the launcher's one holder (J60b).
import type { Bus } from "../mcp/bus.ts";
import { sidecarMessage } from "../mcp/messages.ts";
import { transientDelivery } from "../mcp/codex.ts";
import { startPush, type Deliver, type Held, type Push } from "../mcp/push.ts";

export type PushBinding = {
  runtime: string;
  session: { id: string };
  bindings: { lock(key: string): boolean; save(id: string, name: string, base: string): void };
  // Read each time: a rename replaces them.
  bus: () => Bus;
  env: () => NodeJS.ProcessEnv;
  name: () => string;
  base: () => string;
  log: (s: string) => void;
  warn: (s: string) => void;
  codex?: { deliver(text: string, id: string): Promise<unknown> };
  opencode?: { bind(id: string): void; deliver(text: string, id: string): Promise<unknown> };
  setPush: (push: Push) => void;
};

export function pushBinders(b: PushBinding) {
  // The one place a push starts. The supervisor restarts a push that stopped after a failed delivery, and `held`
  // outlives each one: the message it could not deliver goes in first when the next starts, and a drop reaches the
  // session as a notice (J60b). Every restart goes through here, so none gets a holder of its own.
  const held: Held = { transient: transientDelivery, notice: (s) => b.warn(s) };
  const pushInto = (deliver: Deliver, readerReleaseMs: number): Push =>
    startPush(b.bus(), deliver, b.log, undefined, readerReleaseMs, b.env(), undefined, undefined, held);
  const claim = (id: string) => {
    if (id !== b.session.id) {
      if (!b.bindings.lock(`${b.runtime}:${id}`)) throw new Error("runtime selected an already launched session");
      b.bindings.save(id, b.name(), b.base());
      b.session.id = id;
    }
  };
  const bindThread = (thread: { id: string }, readerReleaseMs = 0) => {
    claim(thread.id);
    b.setPush(pushInto(async e => {
      await b.codex!.deliver(sidecarMessage(e), e.message_id);
    }, readerReleaseMs));
  };
  const bindSession = (id: string, readerReleaseMs = 0) => {
    claim(id);
    b.opencode!.bind(id);
    b.setPush(pushInto(async e => {
      await b.opencode!.deliver(sidecarMessage(e), e.message_id);
    }, readerReleaseMs));
  };
  return { held, bindThread, bindSession };
}
