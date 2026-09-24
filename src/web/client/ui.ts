// The browser side: icons, theme, sidebar, palette, shortcuts, copy, charts.
// It reads the page and /palette.json, and requests nothing else.
declare const lucide: any;
declare const uPlot: any;

const $ = <T extends Element = HTMLElement>(sel: string, root: ParentNode = document) => root.querySelector(sel) as T | null;
const $$ = <T extends Element = HTMLElement>(sel: string, root: ParentNode = document) => [...root.querySelectorAll(sel)] as T[];
const store = {
  get(k: string) { try { return localStorage.getItem(k); } catch { return null; } },
  set(k: string, v: string) { try { localStorage.setItem(k, v); } catch {} },
};

function icons(root?: Element) {
  if (typeof lucide !== "undefined") lucide.createIcons({ attrs: { "stroke-width": 1.75 }, ...(root ? { root } : {}) });
}

// ------------------------------------------------------------------ theme

function currentTheme(): "dark" | "light" {
  const t = document.documentElement.dataset.theme;
  if (t === "dark" || t === "light") return t;
  return matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}
function toggleTheme() {
  const next = currentTheme() === "dark" ? "light" : "dark";
  const apply = () => { document.documentElement.dataset.theme = next; };
  const vt = (document as any).startViewTransition;
  if (vt && !matchMedia("(prefers-reduced-motion: reduce)").matches) vt.call(document, apply); else apply();
  document.cookie = `ab_theme=${next}; Path=/; SameSite=Strict; Max-Age=31536000`;
  window.dispatchEvent(new Event("themechange"));
}

// ------------------------------------------------------------------ sidebar

function initSidebar() {
  document.addEventListener("click", e => {
    const b = (e.target as Element).closest<HTMLElement>("[data-action]");
    if (!b) return;
    switch (b.dataset.action) {
      case "collapse": {
        const c = document.documentElement.dataset.sidebar === "collapsed";
        document.documentElement.dataset.sidebar = c ? "" : "collapsed";
        document.cookie = `ab_sidebar=${c ? "open" : "collapsed"}; Path=/; SameSite=Strict; Max-Age=31536000`;
        break;
      }
      case "drawer": document.body.classList.toggle("drawer-open"); break;
      case "theme": toggleTheme(); break;
      case "palette": openPalette(); break;
    }
  });
  document.addEventListener("click", e => {
    if (document.body.classList.contains("drawer-open") && !(e.target as Element).closest(".sidebar, [data-action=drawer]")) document.body.classList.remove("drawer-open");
  });
}

// ------------------------------------------------------------------ forms

function initForms() {
  document.addEventListener("change", e => {
    const c = (e.target as Element).closest<HTMLElement>("[data-submit-on-change]");
    const f = c?.closest("form");
    if (f) f.requestSubmit();
  });
  // Numbered list fields: a "Line N:" refusal names a line the visitor can see.
  for (const ta of $$<HTMLTextAreaElement>("textarea.lines")) {
    const wrap = ta.closest(".lines-wrap");
    const gutter = wrap?.querySelector<HTMLElement>(".gutter");
    if (!gutter) continue;
    const draw = () => {
      const n = Math.max(ta.value.split("\n").length, Number(ta.rows) || 1);
      const bad = Number(ta.dataset.badLine || 0);
      gutter.innerHTML = Array.from({ length: n }, (_, i) => `<span${i + 1 === bad ? ' class="bad"' : ""}>${i + 1}</span>`).join("");
      gutter.scrollTop = ta.scrollTop;
    };
    ta.addEventListener("input", draw);
    ta.addEventListener("scroll", () => { gutter.scrollTop = ta.scrollTop; });
    draw();
  }
}

// ------------------------------------------------------------------ copy + toast

function initCopy() {
  document.addEventListener("click", async e => {
    const b = (e.target as Element).closest<HTMLElement>("[data-copy]");
    if (!b) return;
    try { await navigator.clipboard.writeText(b.dataset.copy ?? ""); } catch { return; }
    b.classList.add("copied");
    toast("Copied " + (b.dataset.copy ?? ""));
    setTimeout(() => b.classList.remove("copied"), 1200);
  });
  for (const t of $$("[data-toast]")) setTimeout(() => t.classList.add("gone"), 4200);
}

function toast(msg: string) {
  const t = document.createElement("div");
  t.className = "toast";
  t.setAttribute("role", "status");
  t.textContent = msg;
  document.body.appendChild(t);
  setTimeout(() => t.classList.add("gone"), 1800);
  setTimeout(() => t.remove(), 2400);
}

// ------------------------------------------------------------------ palette

type Entry = { title: string; hint?: string; href: string; icon: string; group: string };
let entries: Entry[] | null = null;
let dialog: HTMLDialogElement | null = null;

function pages(): Entry[] {
  return $$<HTMLAnchorElement>(".side-link").map(a => ({ title: a.textContent!.trim(), href: a.getAttribute("href")!, icon: a.querySelector("[data-lucide]")?.getAttribute("data-lucide") ?? "circle", group: "Pages", hint: a.dataset.keys }));
}

async function loadEntries(): Promise<Entry[]> {
  if (entries) return entries;
  const base = pages();
  try {
    const r = await fetch("/palette.json", { credentials: "same-origin", headers: { Accept: "application/json" } });
    if (r.ok) { const j = await r.json(); entries = [...base, ...(j.entries as Entry[])]; return entries; }
  } catch {}
  return base;
}

function score(e: Entry, q: string): number {
  const s = (e.title + " " + (e.hint ?? "")).toLowerCase();
  if (!q) return 1;
  const i = s.indexOf(q);
  if (i === 0) return 100;
  if (i > 0) return 50 - i / 10;
  let j = 0; for (const c of s) if (c === q[j]) j++;
  return j === q.length ? 10 : 0;
}

function openPalette() {
  if (!$(".side-link")) return;
  if (!dialog) {
    dialog = document.createElement("dialog");
    dialog.className = "palette";
    dialog.setAttribute("aria-label", "Jump to");
    dialog.innerHTML = `<div class="palette-box"><div class="palette-input"><i data-lucide="search" class="ic"></i><input type="search" placeholder="Jump to a page, record, user or group…" aria-label="Search" autocomplete="off" spellcheck="false"><kbd>esc</kbd></div><ul class="palette-list" role="listbox"></ul><p class="palette-foot"><kbd>↑</kbd><kbd>↓</kbd> move <kbd>↵</kbd> open <kbd>?</kbd> shortcuts</p></div>`;
    document.body.appendChild(dialog);
    icons(dialog);
    const input = dialog.querySelector("input")!, list = dialog.querySelector("ul")!;
    let shown: Entry[] = [], sel = 0;
    const draw = async () => {
      const all = await loadEntries();
      const q = input.value.trim().toLowerCase();
      shown = all.map(e => [e, score(e, q)] as const).filter(([, s]) => s > 0).sort((a, b) => b[1] - a[1]).slice(0, 40).map(([e]) => e);
      sel = Math.min(sel, Math.max(0, shown.length - 1));
      list.innerHTML = "";
      let group = "";
      shown.forEach((e, i) => {
        if (e.group !== group) { group = e.group; const h = document.createElement("li"); h.className = "palette-group"; h.textContent = group; list.appendChild(h); }
        const li = document.createElement("li");
        li.className = "palette-item" + (i === sel ? " sel" : "");
        li.setAttribute("role", "option");
        li.innerHTML = `<i data-lucide="${e.icon}" class="ic"></i><span class="pt"></span><span class="ph"></span>`;
        li.querySelector(".pt")!.textContent = e.title;
        li.querySelector(".ph")!.textContent = e.hint ?? "";
        li.addEventListener("click", () => { location.href = e.href; });
        list.appendChild(li);
      });
      if (!shown.length) list.innerHTML = `<li class="palette-empty">Nothing you can see matches.</li>`;
      icons(list);
      list.querySelector(".sel")?.scrollIntoView({ block: "nearest" });
    };
    input.addEventListener("input", () => { sel = 0; draw(); });
    input.addEventListener("keydown", e => {
      if (e.key === "ArrowDown") { sel = Math.min(sel + 1, shown.length - 1); draw(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { sel = Math.max(sel - 1, 0); draw(); e.preventDefault(); }
      else if (e.key === "Enter" && shown[sel]) { location.href = shown[sel]!.href; }
    });
    dialog.addEventListener("click", e => { if (e.target === dialog) dialog!.close(); });
    (dialog as any)._draw = draw;
  }
  dialog.showModal();
  const input = dialog.querySelector("input")!;
  input.value = ""; input.focus();
  (dialog as any)._draw();
}

// ------------------------------------------------------------------ shortcuts

function initShortcuts() {
  let g = 0;
  document.addEventListener("keydown", e => {
    const typing = (e.target as Element).closest("input, textarea, select, [contenteditable]");
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); openPalette(); return; }
    if (typing || e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === "/") { const s = $<HTMLInputElement>("input[type=search]"); if (s) { e.preventDefault(); s.focus(); } return; }
    if (e.key === "?") { showKeys(); return; }
    if (e.key === "g") { g = Date.now(); return; }
    if (Date.now() - g < 1200) {
      const link = $$<HTMLAnchorElement>(".side-link").find(a => a.dataset.keys === `g ${e.key}`);
      if (link) location.href = link.href;
      g = 0;
    }
  });
}

function showKeys() {
  let d = $<HTMLDialogElement>("dialog.keys");
  if (!d) {
    d = document.createElement("dialog");
    d.className = "keys palette";
    const rows = [["⌘K / Ctrl-K", "Jump to anything"], ["/", "Search this list"], ["?", "These shortcuts"], ...$$<HTMLAnchorElement>(".side-link").map(a => [a.dataset.keys ?? "", a.textContent!.trim()])];
    d.innerHTML = `<div class="palette-box"><h2>Keyboard shortcuts</h2><dl class="keylist">${rows.map(([k, v]) => `<div><dt>${k!.split(" ").map(x => `<kbd>${x}</kbd>`).join(" ")}</dt><dd>${v}</dd></div>`).join("")}</dl></div>`;
    d.addEventListener("click", e => { if (e.target === d) d!.close(); });
    document.body.appendChild(d);
  }
  d.showModal();
}

// ------------------------------------------------------------------ charts

function cssVar(name: string) { return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }

function initCharts() {
  if (typeof uPlot === "undefined") return;
  for (const host of $$("[data-chart]")) {
    const data = JSON.parse(host.dataset.chart!);
    if (!data.series.length) continue;
    let plot: any;
    const build = () => {
      plot?.destroy();
      const grid = cssVar("--chart-grid"), axis = cssVar("--text-3");
      const w = host.clientWidth || 600;
      const hgt = host.closest(".compact") ? 160 : 260;
      plot = new uPlot({
        width: w, height: hgt,
        cursor: { drag: { x: false, y: false }, points: { size: 7 } },
        legend: { show: false },
        scales: { x: { time: true }, y: { range: (_u: any, _min: number, max: number) => [0, Math.max(1, max * 1.15)] } },
        axes: [
          { stroke: axis, grid: { stroke: grid, width: 1 }, ticks: { stroke: grid }, space: 60, values: (_u: any, ts: number[]) => ts.map(t => { const d = new Date(t * 1000); return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`; }) },
          { stroke: axis, grid: { stroke: grid, width: 1 }, ticks: { show: false }, size: 44 },
        ],
        series: [{ value: (_u: any, t: number) => t == null ? "—" : new Date(t * 1000).toLocaleString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }) },
          ...data.series.map((s: any) => ({
            label: s.label, stroke: cssVar(`--series-${s.key}`), width: 2,
            fill: s.key === "in" ? cssVar("--series-in-fill") : undefined,
            dash: s.key === "out" ? [6, 4] : s.key === "expired" ? [3, 3] : s.key === "refused" ? [1, 3] : undefined,
            points: { show: false },
          }))],
      }, [data.t, ...data.series.map((s: any) => s.values)], host);
    };
    build();
    let last = host.clientWidth;
    new ResizeObserver(() => { if (Math.abs(host.clientWidth - last) > 4) { last = host.clientWidth; build(); } }).observe(host);
    window.addEventListener("themechange", () => setTimeout(build, 30));
  }
}

function start() {
  icons();
  initSidebar();
  initForms();
  initCopy();
  initShortcuts();
  initCharts();
}
if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start); else start();
