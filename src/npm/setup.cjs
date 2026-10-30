"use strict";
// Explicit root-only setup, never an npm lifecycle hook. The installed npm
// packages are trusted via the registry; the private copy prevents later
// source changes from changing the checked release that setup executes.
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");

module.exports = function stage(client, bus) {
  if (process.getuid?.() !== 0) throw new Error("setup requires root");
  // Fixed root-owned parent, outside user-controlled npm prefixes/TMPDIR.
  const root = fs.mkdtempSync("/run/agent-bus-npm-");
  fs.chmodSync(root, 0o700);
  const signals = ["SIGINT", "SIGTERM", "SIGHUP"];
  const onSignal = signal => {
    cleanup();
    process.removeAllListeners(signal);
    process.kill(process.pid, signal);
  };
  const detachSignals = () => { for (const signal of signals) process.removeListener(signal, onSignal); };
  const cleanup = () => { detachSignals(); fs.rmSync(root, {recursive: true, force: true}); };
  for (const signal of signals) process.on(signal, onSignal);
  process.once("exit", cleanup);
  try {
    if (fs.statSync(root).uid !== 0) throw new Error("setup staging directory is not root-owned");
    const manifestPath = path.join(bus, "MANIFEST.sha256");
    if (fs.lstatSync(manifestPath).isSymbolicLink() || fs.statSync(manifestPath).size > 1024 * 1024)
      throw new Error("invalid full release manifest");
    const manifest = fs.readFileSync(manifestPath, "utf8");
    const files = new Map();
    for (const line of manifest.trim().split("\n")) {
      const match = /^([a-f0-9]{64})\s+\*?([^\r\n]+)$/.exec(line);
      if (!match) throw new Error("invalid full release manifest");
      const [, checksum, name] = match;
      if (path.isAbsolute(name) || path.normalize(name) !== name || name.split("/").includes("..") || files.has(name))
        throw new Error("unsafe or repeated release manifest path");
      files.set(name, checksum);
    }
    let total = 0;
    for (const [name, checksum] of files) {
      const sources = [client, bus].filter(base => fs.existsSync(path.join(base, name)));
      if (sources.length !== 1) throw new Error(`missing or duplicated release file: ${name}`);
      const base = sources[0];
      let source = base;
      for (const part of name.split("/")) {
        source = path.join(source, part);
        if (fs.lstatSync(source).isSymbolicLink()) throw new Error("symlink in release files");
      }
      const fd = fs.openSync(source, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW);
      let bytes, mode;
      try {
        const st = fs.fstatSync(fd);
        if (!st.isFile() || st.size > 256 * 1024 * 1024) throw new Error("invalid release file");
        total += st.size;
        if (total > 1024 * 1024 * 1024) throw new Error("release is too large");
        mode = st.mode & 0o111 ? 0o755 : 0o644;
        bytes = fs.readFileSync(fd);
      } finally { fs.closeSync(fd); }
      const destination = path.join(root, name);
      fs.mkdirSync(path.dirname(destination), {recursive: true, mode: 0o755});
      fs.writeFileSync(destination, bytes, {flag: "wx", mode});
      if (crypto.createHash("sha256").update(fs.readFileSync(destination)).digest("hex") !== checksum)
        throw new Error(`release manifest mismatch: ${name}`);
    }
    if (!files.has("agent-bus-setup") || !files.has("agent-busd")) throw new Error("release lacks setup or daemon");
    fs.writeFileSync(path.join(root, "MANIFEST.sha256"), manifest, {flag: "wx", mode: 0o644});
    return {root, cleanup, detachSignals};
  } catch (error) { cleanup(); throw error; }
};
