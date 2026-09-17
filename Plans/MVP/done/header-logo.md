# Header bus mark — 0.5.41

Owner-requested addition to the [shared header](../../../docs/05-discovery.md#what-a-node-says-about-itself), not the broader F.13 redesign.

| Change | Evidence |
|---|---|
| Red bus mark derived from `docs/img/agent-bus.png` | Reference inspected; red double-decker, blue glazing and gold rail retained in a hand-authored SVG. The original artwork is unchanged. |
| No image request or public asset route | Static 971-byte SVG in `logo.go`; decorative beside the AgentBus text, `aria-hidden` and not focusable. |
| Logo spans two rows | 80×58 viewport; identity and counters above navigation on its right. Anonymous pages share the mark without inventing a menu. |
| Identity order | AgentBus version, `@` host, owner, uptime, calls. Existing escaping and unavailable states retained. |
| Header closure | `frameHeader` opens; signed-in `shell` closes after navigation, anonymous template closes directly. |

## Verification

OpenCode reviewed the final two-row source read-only. Web package tests passed.
Browser capture of rendered sign-in and signed-in templates at 1280px:
logo x32, text rows x128; signed-in summary y38.4, navigation y61.8.
Both screenshots inspected (`tmp/logo-41/login-1280.png`, `services-1280.png`).
This is a bounded layout check, not broad browser or accessibility acceptance.

Four targeted overlays fail named web tests: missing logo, exposed decorative
SVG, navigation outside the header, and missing `@` host separator. Logs and
replay: `tmp/logo-41/mutations.log`, `mutate.py`. No full smoke per mutation.

The first separator mutation survived: the fixture mistakenly supplied `@`
inside its hostname and the assertion only matched the hostname. Correcting
both exposed the missing separator. The initial log remains in
`mutations-first.log`. The first full smoke was terminated because its Go
checks predated that correction; only the final rerun is release evidence.

Final frozen `src/smoke.sh --slow`: **585 passed, 0 failed**, exit 0,
vet and race green (`tmp/logo-41/slow-final.log:926`). Tracked script and
`src/logo-final-smoke.local.sh` share SHA-256
`eb96a4415f65a4390ee1f4ebccf52c3c6a01babf64c1a1484b997eb7639512e7`.
