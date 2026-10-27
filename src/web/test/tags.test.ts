import {expect,test} from "bun:test";
import cases from "../../internal/protocol/testdata/tags.json";
import {normalizeTags,recordTags,filterTag} from "../tags.ts";
test("tags refuse invalid characters and match the protocol vectors",()=>{
 for (const c of cases) {if(c.invalid) expect(()=>normalizeTags(c.input)).toThrow("invalid tag");else expect(normalizeTags(c.input)).toEqual(c.expected);}
 expect(recordTags({})).toEqual(["no-tags"]);
 expect(recordTags({personal:true})).toEqual(["personal"]);
 expect(filterTag("BUILD-Notes")).toBe("build-notes");
});

import { readFileSync } from "node:fs";
import { TagBadges } from "../ui/tags.tsx";
import { Pill } from "../ui/kit.tsx";
import { render } from "../jsx.ts";

test("rendered shared badges use colored tag tokens with AA contrast in each theme", () => {
 const html=render(TagBadges({rec:{tags:["messaging"],personal:true}}));
 expect(html).toMatch(/<a class="record-tag"[^>]*>messaging<\/a>/);
 expect(html).toMatch(/<a class="record-tag tag-personal"[^>]*>personal<\/a>/);
 const css=readFileSync(new URL("../style/30-components.css",import.meta.url),"utf8");
 const rule=css.match(/^\.record-tag \{([^}]+)}/m)![1]!;
 expect(rule).toContain("background:var(--tag-bg)");expect(rule).toContain("color:var(--tag-text)");expect(rule).not.toContain("--surface-");
 const personal=css.match(/\.record-tag\.tag-personal \{([^}]+)}/)![1]!;
 expect(personal).toContain("background:var(--tag-personal-bg)");expect(personal).toContain("color:var(--tag-personal-text)");
 const tokens=readFileSync(new URL("../style/00-tokens.css",import.meta.url),"utf8");
 const luminance=(hex:string)=>[0,2,4].map(i=>parseInt(hex.slice(i+1,i+3),16)/255).map(c=>c<=.04045?c/12.92:((c+.055)/1.055)**2.4).reduce((sum,c,i)=>sum+c*[.2126,.7152,.0722][i]!,0);
 for(const selector of [":root {",':root[data-theme="light"] {',":root:not([data-theme]) {"]) {
  const block=tokens.slice(tokens.indexOf(selector)).split("}")[0]!;
  const value=(key:string)=>block.match(new RegExp(`--${key}:\\s*(#[a-f0-9]{6})`))![1]!;
  for(const prefix of ["tag","tag-personal"]) {
   const bg=luminance(value(prefix+"-bg")),fg=luminance(value(prefix+"-text"));
   expect((Math.max(bg,fg)+.05)/(Math.min(bg,fg)+.05)).toBeGreaterThanOrEqual(4.5);
  }
 }
});

test("tag personal and you badges never use alarm tokens or warm alarm hues", () => {
 const css=readFileSync(new URL("../style/30-components.css",import.meta.url),"utf8");
 expect(render(Pill({tone:"accent",children:"you"}))).toBe('<span class="pill tone-accent">you</span>');
 const rule=css.match(/^\.tone-accent \{([^}]+)}/m)![1]!;
 expect(rule).toContain("var(--identity-text)");expect(rule).toContain("var(--identity-bg)");
 const tokens=readFileSync(new URL("../style/00-tokens.css",import.meta.url),"utf8");
 const definitions=(block:string)=>Object.fromEntries([...block.matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)].map(m=>[m[1]!,m[2]!.trim()]));
 const root=definitions(tokens.slice(tokens.indexOf(":root {")).split("}")[0]!);
 for(const selector of [":root {",':root[data-theme="light"] {',":root:not([data-theme]) {"]) {
  const defs={...root,...definitions(tokens.slice(tokens.indexOf(selector)).split("}")[0]!)};
  const color=(key:string):string=>{
   expect(key).not.toMatch(/^(bad|warn|btn-danger)(-|$)/);
   const value=defs[key]!;const alias=/^var\(--([a-z0-9-]+)\)$/.exec(value);
   return alias?color(alias[1]!):value;
  };
  for(const key of ["tag-bg","tag-text","tag-personal-bg","tag-personal-text","identity-bg","identity-text"]) {
   const value=color(key);expect(value).toMatch(/^#[a-f0-9]{6}$/);
   const [r,g,b]=[1,3,5].map(i=>parseInt(value.slice(i,i+2),16));
   expect(r!>g! && r!>b!).toBe(false);
  }
 }
});

test("buttons and active selections share a calm AA primary; secondary stays grey", () => {
 const components=readFileSync(new URL("../style/30-components.css",import.meta.url),"utf8");
 const shell=readFileSync(new URL("../style/20-shell.css",import.meta.url),"utf8");
 for(const [source,selector] of [[components,".btn-primary"],[shell,'.side-link[aria-current="page"]'],[components,'.tab[aria-current="true"]'],[components,'.segmented [aria-current="true"]']]) {
  const body=source!.slice(source!.indexOf(selector!)+selector!.length).split("}")[0]!;
  expect(body).toContain("background: var(--primary-bg)");expect(body).toContain("color: var(--primary-text)");
 }
 const tokens=readFileSync(new URL("../style/00-tokens.css",import.meta.url),"utf8");
 const definitions=(block:string)=>Object.fromEntries([...block.matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)].map(m=>[m[1]!,m[2]!.trim()]));
 const root=definitions(tokens.slice(tokens.indexOf(":root {")).split("}")[0]!);
 const luminance=(hex:string)=>[1,3,5].map(i=>parseInt(hex.slice(i,i+2),16)/255).map(c=>c<=.04045?c/12.92:((c+.055)/1.055)**2.4).reduce((sum,c,i)=>sum+c*[.2126,.7152,.0722][i]!,0);
 for(const selector of [":root {",':root[data-theme="light"] {',":root:not([data-theme]) {"]) {
  const defs={...root,...definitions(tokens.slice(tokens.indexOf(selector)).split("}")[0]!)};
  const color=(key:string):string=>{const value=defs[key]!;const alias=/^var\(--([a-z0-9-]+)\)$/.exec(value);return alias?color(alias[1]!):value;};
  for(const [bg,fg] of [["primary-bg","primary-text"],["primary-hover","primary-text"],["secondary-bg","secondary-text"]]) {
   const a=luminance(color(bg!)),b=luminance(color(fg!));expect((Math.max(a,b)+.05)/(Math.min(a,b)+.05)).toBeGreaterThanOrEqual(4.5);
  }
  const primary=color("primary-bg");const [r,g,b]=[1,3,5].map(i=>parseInt(primary.slice(i,i+2),16));expect(g!).toBeGreaterThan(r!);expect(g!).toBeGreaterThan(b!);
 }
 const pages=readFileSync(new URL("../style/40-pages.css",import.meta.url),"utf8");
 expect(pages).not.toMatch(/\.danger-inline\[open\]\s*>\s*summary\s*\{[^}]*display:\s*none/);
});
