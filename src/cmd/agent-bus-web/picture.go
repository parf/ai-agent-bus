package main

import _ "embed"

// busPicture is the project picture the sign-in page shows. It is in the
// binary because the web child runs with nothing bind-mounted but the binary
// itself, and because this node serves its own assets: the page's own
// Content-Security-Policy allows img-src 'self' and nothing else, so a picture
// hosted anywhere but here does not render at all.
// See docs/05-discovery.md#rules-it-is-built-to.
//
// docs/img/agent-bus.png is the original, 1536x1024 and 2.5MB. This is the
// 648-wide copy, which is the size the page displays.
//
//go:embed agent-bus.jpg
var busPicture []byte
