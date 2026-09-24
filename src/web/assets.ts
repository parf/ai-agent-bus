// Every asset the pages use. External files come from one CDN, each pinned to
// an exact version and carried with its integrity hash; the CSP is generated
// from this table, so a file not listed here cannot load. Our own stylesheet,
// script and picture are served under a content hash and cached for good.
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { createHash } from "node:crypto";

export type External = { url: string; integrity: string };

const CDN = "https://cdn.jsdelivr.net/npm";

export const FONTS = {
  geist: { url: `${CDN}/@fontsource-variable/geist@5.3.0/index.css`, integrity: "sha384-if8MdL2mPpInuz+gfwSqWsp0oVymAAWrxIgU35SlqXZzzjvGAQmx91Y+mMGGibpp", files: `${CDN}/@fontsource-variable/geist@5.3.0/files/` },
  mono: { url: `${CDN}/@fontsource-variable/jetbrains-mono@5.3.0/index.css`, integrity: "sha384-bNykZ+bGB4FclZiYvLmgUfE8clWCvZlLOJ3v63czgIEgamTe0EhkaLXIbptJkvmW", files: `${CDN}/@fontsource-variable/jetbrains-mono@5.3.0/files/` },
};

export const LIBS = {
  lucide: { url: `${CDN}/lucide@1.47.0/dist/umd/lucide.min.js`, integrity: "sha384-v15JX+vZHMR3T6LQR9N6e6wIKrKOdNTbdj8j+e5irqq3L9gCauJz2vGlUzomWyUp" },
  uplot: { url: `${CDN}/uplot@1.6.32/dist/uPlot.iife.min.js`, integrity: "sha384-Gx3t0zdBAuQOuvvmaLZj7HKEiSgWTAs+VdtNY7wt19QDPTDQjFIwAuXDj0zeN00c" },
  uplotCss: { url: `${CDN}/uplot@1.6.32/dist/uPlot.min.css`, integrity: "sha384-IfV0B7MIOYuO95kO9G5ySKPz/85zqFNOAs8iy4tkK5zd9izhJAB8b7lHrwYqqmYE" },
};

export const STYLESHEETS: External[] = [FONTS.geist, FONTS.mono, LIBS.uplotCss];
export const SCRIPTS: External[] = [LIBS.lucide, LIBS.uplot];

export function csp(): string {
  return [
    "default-src 'none'",
    `script-src 'self' ${SCRIPTS.map(s => s.url).join(" ")}`,
    `style-src 'self' ${STYLESHEETS.map(s => s.url).join(" ")}`,
    `font-src ${FONTS.geist.files} ${FONTS.mono.files}`,
    "img-src 'self' data:",
    "connect-src 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
    "base-uri 'none'",
  ].join("; ");
}

export type Local = { path: string; type: string; body: Uint8Array | string; href?: string };

const hash = (b: Uint8Array | string) => createHash("sha256").update(b).digest("hex").slice(0, 12);
const dir = import.meta.dir;

function bundleCss(): string {
  const d = join(dir, "style");
  return readdirSync(d).filter(f => f.endsWith(".css")).sort().map(f => readFileSync(join(d, f), "utf8")).join("\n");
}

function bundleJs(): string {
  const t = new Bun.Transpiler({ loader: "ts", target: "browser", minifyWhitespace: false });
  const src = readFileSync(join(dir, "client/ui.ts"), "utf8");
  return `(()=>{${t.transformSync(src)}})();`;
}

/** Built once at start: hashed names, so a changed file is a new URL. */
export function buildLocal(): { css: Local; js: Local; hero: Local; favicon: Local } {
  const css = bundleCss(), js = bundleJs();
  const hero = readFileSync(join(dir, "assets/agent-bus.webp"));
  const favicon = readFileSync(join(dir, "assets/favicon.svg"), "utf8");
  return {
    css: { path: `/app.${hash(css)}.css`, type: "text/css; charset=utf-8", body: css },
    js: { path: `/ui.${hash(js)}.js`, type: "text/javascript; charset=utf-8", body: js },
    hero: { path: `/agent-bus.${hash(hero)}.webp`, type: "image/webp", body: hero },
    // Browsers keep a favicon long past its page; the hash in the link retires the old one.
    favicon: { path: "/favicon.svg", href: `/favicon.svg?v=${hash(favicon)}`, type: "image/svg+xml; charset=utf-8", body: favicon },
  };
}
