# Outreach

📌 **TL;DR:** translate only the entry funnel (README, landing page, user
guide, glossary) into Chinese, Japanese and Russian, then Korean; announce
where AI-coding-agent developers and self-hosters read. The audience is narrow:
people building with Claude Code, Codex or OpenCode, and self-hosted Linux
users. Owner, 2026-10-29; no release assigned.

## Translation

| Order | Language | Why for this audience |
|---|---|---|
| 1 | Chinese (Simplified) | largest AI-coding-agent community outside the US (Juejin, V2EX, Zhihu); many skip long English docs; needs a Gitee mirror |
| 2 | Japanese | very active Claude Code readers (Zenn, Qiita, X); a native README earns stars |
| 3 | Russian | free to produce; a Habr article reaches readers who otherwise just read English |
| 4 | Korean | smaller but keen Claude Code users (Velog, GeekNews) |
| 5 | Spanish, Portuguese (BR) | large general audience, fewer agent developers; a second wave |
| — | German, French, Hindi | readers use English; not translated |

What to translate, and what not:

| Translate | Keep English only |
|---|---|
| root README, as `README.zh-CN.md`, `README.ja.md`, `README.ru.md` with a language switch at the top | constitution, Plans, changelogs, `src/` READMEs: they change too often |
| the landing page, under a language path per language | |
| `docs/user/README.md`, the zero-to-agent guide | |
| `docs/glossary.md`: short, stable, and it removes the terminology barrier | |

Commands, record names and glyphs stay as they are in every language
(`agent-bus register`, `#worker@srv1`, the kind glyphs).

## Promotion

| Where | Channels |
|---|---|
| Global | Show HN; r/ClaudeAI, r/ChatGPTCoding, r/LocalLLaMA, r/selfhosted; X with Claude Code and MCP tags; pull requests to awesome-mcp-servers and awesome-claude-code lists |
| Chinese | Juejin, V2EX, Zhihu; the Gitee mirror |
| Japanese | a Zenn article on Claude Code sessions talking to each other; X |
| Russian | Habr; a few Telegram channels on AI coding |

Every public post is the owner's own decision and voice; nothing is posted for
him.
