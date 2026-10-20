import { expect, test } from "bun:test";
import { restartOption, RestartBackoff } from "./restart.ts";
test("restart accepts only bounded seconds or minutes and removes its flag", () => {
 expect(restartOption(["a"])).toEqual({args:["a"],delay:undefined});
 expect(restartOption(["--restart","a"])).toEqual({args:["a"],delay:30000});
 expect(restartOption(["--restart=10"] ).delay).toBe(10000);
 expect(restartOption(["--restart=2m"]).delay).toBe(120000);
 for(const bad of ["", "0", "-1", "1s", "1h", "0.1", "Infinity", "999999999999"])
  expect(()=>restartOption([`--restart=${bad}`])).toThrow();
 expect(()=>restartOption(["--restart","--restart=2"])).toThrow();
});
test("restart leaves everything after the separator untouched", () => {
 expect(restartOption(["--", "--restart=0", "--restart"])).toEqual({args:["--", "--restart=0", "--restart"],delay:undefined});
 expect(restartOption(["--restart=2", "--", "--restart=10"])).toEqual({args:["--", "--restart=10"],delay:2000});
});
test("crash backoff doubles, caps, stops at ten, and resets only after a stable TUI", () => {
 let now = 0;
 const policy = new RestartBackoff(30_000, () => now);
 const delays = [30_000,60_000,120_000,240_000,300_000,300_000,300_000,300_000,300_000];
 for (let i=0; i<delays.length; i++) {
   const started = policy.started(); now += 1000;
   expect(policy.failed(started)).toEqual({delay:delays[i],attempt:i+1});
   now += delays[i]!; // waits must not reset the short-lived failure count
 }
 expect(() => policy.failed(policy.started())).toThrow("10 consecutive");
 const started = policy.started(); now += 60_000;
 expect(policy.failed(started)).toEqual({delay:30_000,attempt:1});
 expect(new RestartBackoff(600_000).failed(performance.now())).toEqual({delay:300_000,attempt:1});
});
