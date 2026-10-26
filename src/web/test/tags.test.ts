import {expect,test} from "bun:test";
import cases from "../../internal/protocol/testdata/tags.json";
import {normalizeTags,recordTags,filterTag} from "../tags.ts";
test("tags refuse invalid characters and match the protocol vectors",()=>{
 for (const c of cases) {if(c.invalid) expect(()=>normalizeTags(c.input)).toThrow("invalid tag");else expect(normalizeTags(c.input)).toEqual(c.expected);}
 expect(recordTags({})).toEqual(["no-tags"]);
 expect(recordTags({personal:true})).toEqual(["personal"]);
 expect(filterTag("BUILD-Notes")).toBe("build-notes");
});
