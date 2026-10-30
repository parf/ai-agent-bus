"use strict";
const fs = require("node:fs");
const path = require("node:path");
const {spawn} = require("node:child_process");
const meta = require("./package.json");
const platform = `${process.platform}-${process.arch}`;

function bundle(name, search = __dirname) {
  if (!name) throw new Error(`unsupported platform ${platform}`);
  let filename;
  try { filename = require.resolve(`${name}/package.json`, {paths: [search]}); }
  catch { throw new Error(`the ${name} package is not installed; do not omit optional platform dependencies`); }
  const installed = JSON.parse(fs.readFileSync(filename, "utf8"));
  if (installed.version !== meta.version) throw new Error(`platform version mismatch: expected ${meta.version}`);
  return path.join(path.dirname(filename), "bundle");
}

function child(file, args, cleanup = () => {}, detachSignals = () => {}) {
  const running = spawn(file, args, {stdio: "inherit"});
  for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) process.on(signal, () => running.kill(signal));
  detachSignals();
  process.once("exit", cleanup);
  running.once("error", error => {
    console.error(`agent-bus: could not start program: ${error.code || "error"}`);
    cleanup(); process.exit(127);
  });
  running.once("exit", (code, signal) => {
    cleanup();
    if (signal) { process.removeAllListeners(signal); process.kill(process.pid, signal); }
    else process.exit(code ?? 1);
  });
}

module.exports = function launch(program, args) {
  try {
    const installing = program === "agent-bus-setup" && !args.some(arg => ["--version", "-version", "--help", "-h"].includes(arg));
    if (installing) {
      if (process.getuid?.() !== 0) throw new Error("setup requires explicit sudo; npm install never creates accounts or units");
      const trusted = require("./root-trust.cjs");
      for (const name of [__dirname, __filename, path.join(__dirname, "root-trust.cjs"), path.join(__dirname, "setup.cjs"), path.join(__dirname, "package.json")]) trusted(name);
      trusted(process.argv[1], true);
      trusted(process.execPath, true);
    }
    const clientCommand = ["agent-bus", "agent-bus-runner", "agent-bus-mcp", "ab-claude", "ab-codex", "ab-opencode", "ab-opencode1"].includes(program);
    let client;
    if (meta.agentBus.kind === "bus" && (clientCommand || program === "agent-bus-setup")) {
      const clientPackage = require.resolve(`${meta.agentBus.client}/package.json`);
      if (JSON.parse(fs.readFileSync(clientPackage, "utf8")).version !== meta.version)
        throw new Error("client and bus package versions differ");
      if (installing) require("./root-trust.cjs")(clientPackage);
      client = bundle(meta.agentBus.clientPlatforms[platform], path.dirname(clientPackage));
    }
    const root = meta.agentBus.kind === "bus" && clientCommand ? client : bundle(meta.agentBus.platforms[platform]);
    if (installing) {
      if (process.getuid?.() !== 0) throw new Error("setup requires explicit sudo; npm install never creates accounts or units");
      const staged = require("./setup.cjs")(client, root);
      child(path.join(staged.root, "agent-bus-setup"), args, staged.cleanup, staged.detachSignals);
      return;
    }
    const compiled = program === "agent-bus-mcp";
    const file = compiled ? "agent-bus-ts" : program.startsWith("ab-") ? `launchers/${program}` : program;
    child(path.join(root, file), compiled ? ["mcp", ...args] : args);
  } catch (error) {
    console.error(`agent-bus: ${error.message}`);
    process.exitCode = 1;
  }
};
