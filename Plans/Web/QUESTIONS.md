# Questions Web face rewrite

📌 **TL;DR:** One owner choice before W.1. Each states the recommendation
first; the [README](README.md#web-face-rewrite) is written on those
recommendations and changes if the answer differs.

## Open questions

### Q117 CSP

| Option | |
|---|---|
| **A (recommended)** | `style-src 'self'` with a hashed stylesheet (no inline styles), `font-src 'self'`, `connect-src 'self'` for the palette's `/palette.json` |
| B | Keep the current policy: inline styles, no `connect-src`, no palette data |
