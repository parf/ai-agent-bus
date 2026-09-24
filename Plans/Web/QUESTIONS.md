# Questions Web face rewrite

📌 **TL;DR:** One open choice, settled on the W.2 style guide; Q116–Q120 are
in [DECISIONS](DECISIONS.md#decisions). A new choice found while building is
added here with its recommendation first.

## Open questions

### Q121 Icons

Emoji marks (🏠 👾 📮 …) differ by operating system and read dated on a dark
console. The canonical glyphs live in `src/internal/display`, shared with the
CLI.

| Option | |
|---|---|
| **A (recommended)** | The web face draws Lucide icons for navigation, kinds and authority; each maps one-to-one to a `display` glyph, so the CLI keeps its emoji and the meaning stays single-sourced. Shown side by side with B on the style guide |
| B | Keep the emoji on the web as well |
