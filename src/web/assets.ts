// Vendored, version-pinned assets: every browser resource is same-origin.
// License texts live in THIRD-PARTY-NOTICES.md.
import { readFileSync, readdirSync } from "node:fs";
import { basename, join } from "node:path";
import { createHash } from "node:crypto";

export function csp(): string {
  return ["default-src 'none'", "script-src 'self'", "style-src 'self'",
    "font-src 'self'", "img-src 'self' data:", "connect-src 'self'",
    "form-action 'self'", "frame-ancestors 'none'", "base-uri 'none'"].join("; ");
}
export type Local = { path: string; type: string; body: Uint8Array | string; href?: string };
const hash = (b: Uint8Array | string) => createHash("sha256").update(b).digest("hex").slice(0, 12);
const dir = import.meta.dir, vendor = join(dir, "assets/vendor");
const fonts = new Map<string,Local>();
let fontCss = "";
for (const family of ["geist", "jetbrains-mono"]) {
  const folder = join(vendor, family);
  for (const file of readdirSync(folder).filter(f => f.endsWith(".woff2")).sort()) {
    const body = readFileSync(join(folder,file));
    fonts.set(file,{path:`/${basename(file,".woff2")}.${hash(body)}.woff2`,type:"font/woff2",body});
  }
  fontCss += readFileSync(join(folder,"index.css"),"utf8").replace(/url\(['"]?([^'"\)]+)['"]?\)/g, (_match, ref: string) => {
    const font = fonts.get(basename(ref));
    if (!font) throw Error(`missing bundled font ${ref}`);
    return `url('${font.path}')`;
  });
}
function bundleCss(): string {
  const d = join(dir, "style");
  return fontCss + "\n" + readFileSync(join(vendor,"uPlot.min.css"),"utf8") + "\n" +
    readdirSync(d).filter(f => f.endsWith(".css")).sort().map(f => readFileSync(join(d, f), "utf8")).join("\n");
}
async function bundleJs(): Promise<string> {
  const result = await Bun.build({entrypoints:[join(dir,"client/ui.ts")],target:"browser",format:"iife"});
  if (!result.success) throw new AggregateError(result.logs,"The browser UI could not be bundled");
  return result.outputs[0]!.text();
}
const browserJs = await bundleJs();
/** Built once at start; content hashes allow immutable browser caching. */
export function buildLocal() {
  const css=bundleCss(),js=browserJs;
  const hero=readFileSync(join(dir,"assets/agent-bus.webp"));
  const favicon=readFileSync(join(dir,"assets/favicon.svg"),"utf8");
  const lucide=readFileSync(join(dir,"assets/lucide.min.js"),"utf8");
  const uplot=readFileSync(join(vendor,"uPlot.iife.min.js"),"utf8");
  return {
    css:{path:`/app.${hash(css)}.css`,type:"text/css; charset=utf-8",body:css},
    js:{path:`/ui.${hash(js)}.js`,type:"text/javascript; charset=utf-8",body:js},
    hero:{path:`/agent-bus.${hash(hero)}.webp`,type:"image/webp",body:hero},
    favicon:{path:"/favicon.svg",href:`/favicon.svg?v=${hash(favicon)}`,type:"image/svg+xml; charset=utf-8",body:favicon},
    lucide:{path:`/lucide.${hash(lucide)}.js`,type:"text/javascript; charset=utf-8",body:lucide},
    uplot:{path:`/uplot.${hash(uplot)}.js`,type:"text/javascript; charset=utf-8",body:uplot},
    fonts:[...fonts.values()],
  };
}
