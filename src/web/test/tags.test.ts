import {expect,test} from "bun:test";
import cases from "../../internal/protocol/testdata/tags.json";
import {normalizeTags,recordTags,filterTag} from "../tags.ts";
test("tags use commas only, preserve internal spaces and normalize identically to the daemon",()=>{
 for (const c of cases) expect(normalizeTags(c.input)).toEqual(c.expected);
 expect(normalizeTags("  BUILD Notes , Claude,CLAUDE, no-tags,  x_ y ,c++ ,v2, 0.8  ")).toEqual(["build notes","c","claude","v","x_ y"]);
 expect(normalizeTags(" a  b, alpha\nbeta, x\ty ")).toEqual(["a b", "alpha beta", "x y"]);
 expect(recordTags({})).toEqual(["no-tags"]);
 expect(recordTags({personal:true})).toEqual(["personal"]);
 expect(filterTag(" BUILD Notes ")).toBe("build notes");
});
