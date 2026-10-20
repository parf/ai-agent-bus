import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

export async function restartChecks(launcher: string, cwd: string, env: NodeJS.ProcessEnv, check: (name: string, ok: boolean) => void) {
 const runtime = env.TEST_RUNTIME!;
 const rows = (file: string): any[] => existsSync(file) ? readFileSync(file,"utf8").split("\n").filter(Boolean).map(l=>JSON.parse(l)) : [];
 const until = async (f:()=>boolean, ms=12000) => { const end=Date.now()+ms; while(!f() && Date.now()<end) await Bun.sleep(20); return f(); };
 const launch = (suffix:string, flags:string[], extra:NodeJS.ProcessEnv={}) => {
  const file=env.TEST_EVENTS+"-restart-"+suffix;
  const state=env.XDG_STATE_HOME+"-restart-"+suffix;
  const proc=Bun.spawn([launcher,...flags,"restart prompt"],{cwd,env:{...env,XDG_STATE_HOME:state,TEST_EVENTS:file,TEST_RESTART_CHECK:"1",TEST_HOLD:"",...extra,AGENT_BUS_NAME:`#${runtime}/restart-${process.pid}-${suffix}@srv1`},stdout:"pipe",stderr:"pipe"});
  let output="";
  const capture=async(stream:ReadableStream<Uint8Array>)=> {for await(const data of stream) output+=new TextDecoder().decode(data);};
  const captured=Promise.all([capture(proc.stdout),capture(proc.stderr)]);
  return {proc,file,state,output:()=>output,captured};
 };
 const stop=async(r:ReturnType<typeof launch>)=>{
  if(r.proc.exitCode===null) r.proc.kill("SIGTERM");
  await r.proc.exited; await r.captured;
  const pids=rows(r.file).filter(e=>e.kind==="argv" || e.kind==="mcp-pid").map(e=>e.kind==="mcp-pid"?e.data:e.pid);
  const alive=()=>pids.filter(pid=>{try{process.kill(pid,0);return true;}catch{return false;}});
  await until(()=>alive().length===0,2000);
  check(`${runtime} restart cleanup leaves no runtime children`,alive().length===0);
  for(const pid of alive()) try{process.kill(pid,"SIGKILL");}catch{}
 };
 const r=launch("three",["--restart=1"],{TEST_CRASH_SIGNAL:"1"});
 try {
  const ready=await until(()=>rows(r.file).filter(e=>e.kind==="identity").length>=3 || r.proc.exitCode!==null);
  const all=rows(r.file), identities=all.filter(e=>e.kind==="identity"), exits=all.filter(e=>e.kind==="runtime-exit");
  check(`${runtime} SIGKILL restarts three times under one fixed name`,ready && identities.length===3 && new Set(identities.map(e=>e.data.name)).size===1);
  check(`${runtime} restart observes a non-core-dumping signal`,(r.output().match(/exited \(SIGKILL\)/g) ?? []).length===2);
  const tuis=all.filter(e=>e.kind==="argv" && e.data.includes("restart prompt"));
  check(`${runtime} restart retains user arguments and keeps launcher flags private`,tuis.length===3 && tuis.every(e=>!e.data.some((a:string)=>a.startsWith("--restart"))));
  const sessionFlag = runtime === "claude" ? "--resume" : runtime === "codex" ? "resume" : "--session";
  const expectedSession = runtime === "claude" ? "11111111-1111-4111-8111-111111111111" : `${runtime}-session`;
  check(`${runtime} restart continues the same session`,tuis.length===3 && tuis.slice(1).every(e=>e.data[e.data.indexOf(sessionFlag)+1]===expectedSession));
  check(`${runtime} restart waits before each relaunch`,exits.length===2 && tuis.length===3 && tuis.slice(1).every((e,i)=>e.at-exits[i].at>=1000));
  if(runtime!=="claude") check(`${runtime} restart reuses its one background server`, all.filter(e=>e.kind==="argv" && ["serve","app-server"].includes(e.data[0])).length===1);
  if(identities.length!==3) console.log(r.output());
 } finally { await stop(r); }
 const waiting=launch("interrupt",["--restart"]);
 try {
  check(`${runtime} restart announces the default wait`,await until(()=>waiting.output().includes("restarting in 30s — Ctrl-C to stop")));
  check(`${runtime} keeps the name locks during the wait`,readdirSync(join(waiting.state,"agent-bus/sessions")).some(f=>f.endsWith(".lock")));
  waiting.proc.kill("SIGINT");
  const done=await until(()=>waiting.proc.exitCode!==null,3000);
  check(`${runtime} Ctrl-C during restart wait ends without another start`,done && await waiting.proc.exited===130 && rows(waiting.file).filter(e=>e.kind==="identity").length===1);
  check(`${runtime} Ctrl-C releases launcher locks`,done && !readdirSync(join(waiting.state,"agent-bus/sessions")).some(f=>f.endsWith(".lock")));
 } finally { await stop(waiting); }
 const clean=launch("clean",["--restart=1"],{TEST_EXIT:"0"});
 try {
  const done=await until(()=>clean.proc.exitCode!==null,6000);
  check(`${runtime} clean exit never restarts`,done && await clean.proc.exited===0 && rows(clean.file).filter(e=>e.kind==="identity").length===1 && !clean.output().includes("restarting in"));
 } finally {await stop(clean);}
 if(runtime!=="claude") {
  const server=launch("server",["--restart=1"],{TEST_KILL_SERVER:"1"});
  try {
   const done=await until(()=>server.proc.exitCode!==null,6000);
   check(`${runtime} background server exit ends without restart`,done && await server.proc.exited!==0 && !server.output().includes("restarting in") && rows(server.file).filter(e=>e.kind==="server-ready").length===1);
  } finally {await stop(server);}
 }

 const once=launch("once",[]);
 try {
  const done=await until(()=>once.proc.exitCode!==null,6000);
  check(`${runtime} no restart flag exits once`,done && await once.proc.exited===17 && rows(once.file).filter(e=>e.kind==="identity").length===1);
 } finally {await stop(once);}
}
