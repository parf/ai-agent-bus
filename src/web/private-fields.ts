import { type Ctx, type Rec } from "./ctx.ts";
/** Read only as the visitor. A failed private read must not block other edits. */
export async function storedPrivateFields(ctx: Ctx, rec: Rec): Promise<Record<string,string>> {
  const stored: Record<string,string> = {};
  await Promise.all((["secret", "config"] as const).map(async field => {
    if (!rec[`${field}_sha`]) { stored[field]=""; return; }
    try {
      const value = await ctx.bus<string>("GET",`/${field}`,{query:{name:rec.name},raw:true});
      stored[field] = field === "config" ? value.trim() : value;
    } catch {
      stored[field]="";
      stored[`_${field}_error`]="The stored value could not be read. Leave blank to keep it.";
    }
  }));
  return stored;
}
