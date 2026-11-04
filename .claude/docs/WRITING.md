# How we write docs

📌 **TL;DR:** Every page tells the reader **why** the thing exists and **what they get** before **how** it works. Short bullets, one idea each, the essentials in the first 10–20% of the page, real examples tucked away, technical detail last.

## 🎯 The order of every page

| # | Part | Answers | Form |
|---|---|---|---|
| 1 | **Why** | What problem does this solve, and for whom? | 1–3 bullets, plain words |
| 2 | **What you get** | What can the reader do now that they couldn't before? | short bullets, one outcome each |
| 3 | **How to use it** | The one way to start, then real examples | a few commands; more in closed `<details>` |
| 4 | **Technical details** | Internals, formats, limits | last, in `<details>` when long |

- A reader who stops after the first 10–20% knows why it exists, what they get and how to start.
- Never open with the mechanism. The component name is the *how*, not the headline.

## 🧭 Voice

- **Executive view first.** Real use cases, outcomes, business value.
- **Show the scale.** This is an enterprise platform, not a small tool: it runs on one laptop and grows to many hosts, users and teams. Say so early.
- **Benefits, not internals.** Nobody cares about our database, daemon, broker, registry, queues, wire format or programming languages. Name them only as a one-line proof under a point, never as the point.
- **Plain words.** "Your AI uses mail without ever seeing the password", not "credentials are held by a helper record with an ACL".
- **Present tense, release scope.** Product pages describe the release as shipped. No "coming", no "not built", no future plans. What is outside the release is simply not mentioned.
- **Lead with the standard.** Pick the example most readers use (Slack first, then Telegram, SMS).

## ✂️ Form

- **No walls of text.** Bullets over paragraphs; anything longer than ~3 lines becomes bullets or a table.
- **One simple idea per bullet**, understandable at a glance.
- **Glyphs** on product pages: one per point, to make bullets scannable.
- **Simple Mermaid diagrams** where a picture explains faster: few boxes, plain labels.
- **Tables** for comparisons and options.
- **Keep images.** Never drop an existing image or asset in a rewrite without saying so; never show the same image twice.

## 🧪 Examples

- **Real only.** Every command and output was actually run; never invented.
- **One line, then the command.** Say what it does in one plain line, then the command. No trailing `#` caveat comments.
  - List all visible records in a readable table:
    `agent-bus ls -h`
- Put longer example sessions in closed `<details>` blocks.

## ✅ Truth

- Every claim is true for the release it describes; check it against the code or the release plan.
- If the wording oversells (e.g. "automatically" when it takes one call), reword to the truth, don't drop the benefit.
- End-user guides carry no build status, versions or plan links.

## 🔍 Checklist for a page

- [ ] Why → what → how → tech, in that order
- [ ] Essentials in the first 10–20%
- [ ] No jargon or internals in the opening
- [ ] Bullets, no walls of text
- [ ] Examples real and run; one line + command
- [ ] No future references on release pages
- [ ] Links and images intact; no duplicates
