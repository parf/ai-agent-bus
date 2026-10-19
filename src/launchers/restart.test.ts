import { expect, test } from "bun:test";
import { restartOption } from "./restart.ts";
test("restart accepts only bounded seconds or minutes and removes its flag", () => {
 expect(restartOption(["a"])).toEqual({args:["a"],delay:undefined});
 expect(restartOption(["--restart","a"])).toEqual({args:["a"],delay:30000});
 expect(restartOption(["--restart=10"] ).delay).toBe(10000);
 expect(restartOption(["--restart=2m"]).delay).toBe(120000);
 for(const bad of ["", "0", "-1", "1s", "1h", "0.1", "Infinity", "999999999999"])
  expect(()=>restartOption([`--restart=${bad}`])).toThrow();
 expect(()=>restartOption(["--restart","--restart=2"])).toThrow();
});
