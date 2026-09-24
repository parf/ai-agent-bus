# Questions Web face rewrite

📌 **TL;DR:** Five owner choices before W.1. Each states the recommendation
first; the [README](README.md#web-face-rewrite) is written on those
recommendations and changes if the answer differs.

## Open questions

### Q116 Confinement

The Go child is confined by the supervisor's bubblewrap and cgroup. A face
under its own account needs its own.

| Option | |
|---|---|
| **A (recommended)** | Its own systemd unit with sandboxing and resource limits ([process and account](README.md#process-and-account)); the supervisor drops `-web`, bubblewrap and the cgroup code |
| B | Keep supervising it from `agent-busd` under bubblewrap, switching the user inside the sandbox |

### Q117 CSP

| Option | |
|---|---|
| **A (recommended)** | `style-src 'self'` with a hashed stylesheet (no inline styles), `font-src 'self'`, `connect-src 'self'` for the palette's `/palette.json` |
| B | Keep the current policy: inline styles, no `connect-src`, no palette data |

### Q118 Fonts

| Option | |
|---|---|
| **A (recommended)** | Self-hosted Inter and JetBrains Mono, about 400 KB, cached a day |
| B | System font stack only |

### Q119 Behaviour changes

| Option | |
|---|---|
| **A (recommended)** | Fix every listed item ([behaviour changes](README.md#behaviour-changes)); the spec files are updated to the new behaviour as each step lands |
| B | Exact parity first, fixes after cutover |

### Q120 Release line

| Option | |
|---|---|
| **A (recommended)** | The rewrite is the `0.9.x` odd line: development may break the web face; `STABLE` when W.11 passes, then `0.10` |
| B | Patch bumps on `0.8.x` |
