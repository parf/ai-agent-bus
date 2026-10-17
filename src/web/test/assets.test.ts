import {expect,test} from "bun:test";
import {buildLocal} from "../assets.ts";

test("the shipped UI asset parses as a classic script with its tag helper bundled",()=>{
 const js=String(buildLocal().js.body);
 expect(()=>new Function(js)).not.toThrow();
 expect(js).toContain("normalizeTags");
 expect(js).toContain('"no-tags"');
 expect(new Bun.Transpiler({loader:"js"}).scanImports(js)).toEqual([]);
});
