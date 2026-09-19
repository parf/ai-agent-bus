package main

// The red double-decker, blue glazing and gold rail come from
// docs/img/agent-bus.png. A small inline mark keeps the public sign-in page
// independent of an asset route; the adjacent AgentBus text names it.
const nodeLogo = `<svg class=node-logo width="80" height="58" viewBox="0 0 72 52" aria-hidden="true" focusable="false" xmlns="http://www.w3.org/2000/svg">
<g transform="translate(72 0) scale(-1 1)">
<path d="M12 7h47a6 6 0 0 1 6 6v27a3 3 0 0 1-3 3H8a3 3 0 0 1-3-3l2-26a7 7 0 0 1 5-7Z" fill="#e83b32" stroke="#172b3a" stroke-width="2.5"/>
<path d="M11 12h9v12H10Zm14 0h9v10h-9Zm14 0h9v10h-9Zm14 0h7v10h-7Z" fill="#bce6ef" stroke="#172b3a" stroke-width="2" stroke-linejoin="round"/>
<path d="M10 28h10v10H9Zm16 0h10v9H26Zm15 0h10v9H41Zm15 0h5v12h-5Z" fill="#3d718e" stroke="#172b3a" stroke-width="2" stroke-linejoin="round"/>
<path d="M24 25h37" stroke="#ffcf57" stroke-width="2.5" stroke-linecap="round"/>
<path d="M8 41h55" stroke="#a72228" stroke-width="3"/>
<circle cx="19" cy="43" r="6" fill="#172b3a"/><circle cx="19" cy="43" r="2.5" fill="#c7d5db"/>
<circle cx="54" cy="43" r="6" fill="#172b3a"/><circle cx="54" cy="43" r="2.5" fill="#c7d5db"/>
<path d="M7 37h4" stroke="#ffdf83" stroke-width="3" stroke-linecap="round"/>
</g>
</svg>`

// faviconSVG is the same bus on a square plate, drawn again rather than scaled:
// the sign-in mark's windows and wheels disappear at 16px, which is the only
// size a tab ever shows. Repository-owned and self-contained, like the page
// title images (Plans/MVP/web/glyphs.md#page-title-images-and-glyphs).
const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="32" height="32">
<rect width="32" height="32" rx="7" fill="#172b3a"/>
<path d="M5 11a4 4 0 0 1 4-4h14a4 4 0 0 1 4 4v10H5Z" fill="#e83b32"/>
<path d="M8 11h5v5H8Zm7 0h9v5h-9Z" fill="#bce6ef"/>
<path d="M5 19h22" stroke="#ffcf57" stroke-width="2"/>
<circle cx="11" cy="23" r="3" fill="#c7d5db"/>
<circle cx="21" cy="23" r="3" fill="#c7d5db"/>
</svg>`
