import {expect,test} from "bun:test";
import {storedPrivateFields} from "../private-fields.ts";
import type {Ctx,Rec} from "../ctx.ts";

test("a failed private read leaves other record fields editable and keeps the other private value",async()=>{
 const ctx={bus:async(_method:string,path:string)=>{if(path==="/secret")throw new Error("PRIVATE upstream text");return '{"large":9007199254740993}\n';}} as unknown as Ctx;
 const got=await storedPrivateFields(ctx,{name:"#test@host",secret_sha:"s",config_sha:"c"} as Rec);
 expect(got.secret).toBe("");
 expect(got.config).toBe('{"large":9007199254740993}');
 expect(got._secret_error).toContain("Leave blank to keep it");
 expect(JSON.stringify(got)).not.toContain("PRIVATE upstream text");
});
