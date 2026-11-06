# OpenRouter models, Nov 5 2026

📌 **TL;DR:** every model OpenRouter offers through `#ai`, by kind and vendor. ⭐ marks each vendor's strongest current model (its newest top-priced release); the newest version of every other model line follows, dearest first, and older versions and model lines with no release in the vendor's last six months are folded. `#ai`'s default is marked (default). Details for one: `ab ai model <id>`.

Source: OpenRouter `/api/v1/models` and `/api/v1/images/models`, fetched 2026-11-05 (454 text and media entries, 63 image models). Prices are US$ per million tokens; variants (`:free`, `:batch`, `:thinking`) and `~` aliases are noted on their base model.

# Which to use

| Need | Start with |
|---|---|
| Routine bus asks: summaries, short answers, classification | `anthropic/claude-haiku-5.5` (the `#ai` default) or `openai/gpt-6-luna` |
| Harder reasoning, careful writing, code review | `anthropic/claude-sonnet-5.5`, `openai/gpt-6.1-sol` |
| The hardest problems, price no object | ⭐ of Anthropic, OpenAI or Google below |
| Very cheap bulk work | DeepSeek, Qwen, GLM FlashX, or anything marked free |
| Images or PDFs as input | any row whose Input lists image or file |
| Making images | the Image generation section |

# Always-latest aliases

Each routes to the vendor's newest model of that line at request time, e.g. `ab ai ask` with `"model":"~anthropic/claude-sonnet-latest"`.

`~anthropic/claude-fable-latest` · `~anthropic/claude-haiku-latest` · `~anthropic/claude-opus-latest` · `~anthropic/claude-sonnet-latest` · `~deepseek/deepseek-flash-latest` · `~deepseek/deepseek-pro-latest` · `~deepseek/deepseek-v4-flash-latest` · `~google/gemini-flash-latest` · `~google/gemini-pro-latest` · `~moonshotai/kimi-latest` · `~openai/gpt-astra-latest` · `~openai/gpt-luna-latest` · `~openai/gpt-mini-latest` · `~openai/gpt-sol-latest` · `~openai/gpt-terra-latest` · `~unbiased/pareto-latest` · `~x-ai/grok-latest` · `~z-ai/glm-flash-latest` · `~z-ai/glm-latest`

# Text and chat

## anthropic (15)

Anthropic's Claude: strong at coding, agents, careful writing and following instructions; Fable is the top tier, then Opus, Sonnet, Haiku (fastest, cheapest).

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `anthropic/claude-fable-5.1` · :batch | Claude Fable 5.1 improves on Claude Fable 5 across the board, with the biggest gains in agentic coding, long-running agentic workflows, and knowledge… | 1M | $10.00 | $50.00 | text, image, file | 2026-09-01 |
| `anthropic/claude-opus-5.5` · :batch | Anthropic's flagship model for demanding reasoning, coding, and long-horizon agentic work, succeeding Claude Opus 5 | 1M | $4.00 | $20.00 | text, image, file | 2026-09-22 |
| `anthropic/claude-sonnet-5.5` · :batch | Anthropic's Sonnet-class model for well-scoped everyday work, succeeding Claude Sonnet 5 as a direct upgrade | 1M | $2.00 | $10.00 | text, image, file | 2026-09-28 |
| **default** `anthropic/claude-haiku-5.5` · :batch | Anthropic's small, fast model for high-volume, cost-sensitive work such as summarization, subagents, and browser use | 1M | $0.100 | $0.50 | text, image, file | 2026-10-07 |

<details><summary>Older anthropic versions and lines (11)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `anthropic/claude-opus-5` · :batch | Anthropic’s flagship model for demanding reasoning, coding, and long-horizon agentic work | 1M | $5.00 | $25.00 | text, image, file | 2026-07-24 |
| `anthropic/claude-sonnet-5` · :batch | Sonnet 5 is Anthropic's most capable Sonnet-class model, with frontier performance across coding, agents, and professional work | 1M | $2.00 | $10.00 | text, image, file | 2026-06-30 |
| `anthropic/claude-fable-5` · :batch | A Mythos-class model from Anthropic, built for autonomous knowledge work and coding | 1M | $10.00 | $50.00 | text, image, file | 2026-06-09 |
| `anthropic/claude-opus-4.8` · :batch | Anthropic's most capable generally available model in the Opus family | 1M | $5.00 | $25.00 | text, image, file | 2026-05-27 |
| `anthropic/claude-opus-4.7` · :batch | Opus 4.7 is the next generation of Anthropic's Opus family, built for long-running, asynchronous agents | 1M | $5.00 | $25.00 | text, image, file | 2026-04-16 |
| `anthropic/claude-sonnet-4.6` · :batch | Sonnet 4.6 is Anthropic's most capable Sonnet-class model yet, with frontier performance across coding, agents, and professional work | 1M | $3.00 | $15.00 | text, image, file | 2026-02-17 |
| `anthropic/claude-opus-4.6` · :batch | Opus 4.6 is Anthropic’s strongest model for coding and long-running professional tasks | 1M | $5.00 | $25.00 | text, image, file | 2026-02-04 |
| `anthropic/claude-opus-4.5` · :batch | Anthropic’s frontier reasoning model optimized for complex software engineering, agentic workflows, and long-horizon computer use | 200K | $5.00 | $25.00 | file, image, text | 2025-11-24 |
| `anthropic/claude-haiku-4.5` · :batch | Anthropic’s fastest and most efficient model, delivering near-frontier intelligence at a fraction of the cost and latency of larger Claude models | 200K | $1.00 | $5.00 | text, image, file | 2025-10-15 |
| `anthropic/claude-sonnet-4.5` · :batch | Anthropic’s most advanced Sonnet model to date, optimized for real-world agents and coding workflows | 1M | $3.00 | $15.00 | text, image, file | 2025-09-29 |
| `anthropic/claude-opus-4.1` · :batch | An updated version of Anthropic’s flagship model, offering improved performance in coding, reasoning, and agentic tasks | 200K | $15.00 | $75.00 | image, text, file | 2025-08-05 |

</details>

## openai (56)

OpenAI's GPT models: general chat, coding and reasoning; "pro" variants think longer for harder problems.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `openai/gpt-6-astra` · :batch | OpenAI's flagship model for demanding end-to-end work | 1.1M | $10.00 | $50.00 | file, image, text | 2026-09-04 |
| `openai/gpt-6-astra-pro` · :batch | The same underlying model as GPT-6 Astra, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $10.00 | $50.00 | file, image, text | 2026-09-04 |
| `openai/gpt-chat-latest` | GPT Chat Latest points to OpenAI's stable API alias `chat-latest` that always resolves to the latest Instant chat model used in ChatGPT | 400K | $5.00 | $30.00 | text, image, file | 2026-05-05 |
| `openai/gpt-6.1-sol-pro` · :batch | The same underlying model as GPT-6.1 Sol, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-29 |
| `openai/gpt-6.1-sol` · :batch | An upgrade to GPT-6 Sol from OpenAI, positioned below the flagship GPT-6 Astra in the GPT-6 series | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-29 |
| `openai/gpt-6.1-terra-pro` · :batch | The same underlying model as GPT-6.1 Terra, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $1.00 | $5.00 | file, image, text | 2026-10-26 |
| `openai/gpt-6.1-terra` · :batch | The mid-tier model in OpenAI's GPT-6.1 series, priced below GPT-6.1 Sol | 1.1M | $1.00 | $5.00 | file, image, text | 2026-10-26 |
| `openai/gpt-6-luna-pro` · :batch | The same underlying model as GPT-6 Luna, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $0.100 | $0.50 | file, image, text | 2026-09-22 |
| `openai/gpt-6-luna` · :batch | The fast, cost-efficient model in OpenAI's GPT-6 series, positioned below GPT-6 Sol | 1.1M | $0.100 | $0.50 | file, image, text | 2026-09-22 |

<details><summary>Older openai versions and lines (47)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `openai/gpt-6-sol-pro` · :batch | The same underlying model as GPT-6 Sol, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-22 |
| `openai/gpt-6-sol` · :batch | The cost-efficient high-end model in OpenAI's GPT-6 series, positioned below the flagship GPT-6 Astra and above the fast GPT-6 Luna tier | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-22 |
| `openai/gpt-5.6-luna-pro` · :batch | The same underlying model as GPT-5.6 Luna, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $0.20 | $1.20 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-luna` · :batch | A fast, cost-efficient model in OpenAI's GPT-5.6 series | 1.1M | $0.20 | $1.20 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-terra-pro` · :batch | The same underlying model as GPT-5.6 Terra, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $2.00 | $12.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-terra` · :batch | A balanced model in OpenAI's GPT-5.6 series, positioned between the flagship Sol tier and the cost-efficient Luna tier | 1.1M | $2.00 | $12.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-sol-pro` · :batch | The same underlying model as GPT-5.6 Sol, served with `reasoning.mode` set to `pro` for higher-quality responses on complex tasks | 1.1M | $4.00 | $20.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-sol` · :batch | The flagship model in OpenAI's GPT-5.6 series | 1.1M | $4.00 | $20.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.5-pro` · :batch | OpenAI’s high-capability model optimized for deep reasoning and accuracy on complex, high-stakes workloads | 1.1M | $30.00 | $180.00 | file, image, text | 2026-04-24 |
| `openai/gpt-5.5` · :batch | OpenAI’s frontier model designed for complex professional workloads, building on GPT-5.4 with stronger reasoning, higher reliability, and improved… | 1.1M | $5.00 | $30.00 | file, image, text | 2026-04-24 |
| `openai/gpt-5.4-nano` · :batch | GPT-5.4 nano is the most lightweight and cost-efficient variant of the GPT-5.4 family, optimized for speed-critical and high-volume tasks | 400K | $0.20 | $1.25 | file, image, text | 2026-03-17 |
| `openai/gpt-5.4-mini` · :batch | GPT-5.4 mini brings the core capabilities of GPT-5.4 to a faster, more efficient model optimized for high-throughput workloads | 400K | $0.75 | $4.50 | file, image, text | 2026-03-17 |
| `openai/gpt-5.4-pro` · :batch | OpenAI's most advanced model, building on GPT-5.4's unified architecture with enhanced reasoning capabilities for complex, high-stakes tasks | 1.1M | $30.00 | $180.00 | text, image, file | 2026-03-05 |
| `openai/gpt-5.4` · :batch | OpenAI’s latest frontier model, unifying the Codex and GPT lines into a single system | 1.1M | $2.50 | $15.00 | text, image, file | 2026-03-05 |
| `openai/gpt-5.3-codex` | OpenAI’s most advanced agentic coding model, combining the frontier software engineering performance of GPT-5.2-Codex with the broader reasoning and… | 400K | $1.75 | $14.00 | text, image, file | 2026-02-24 |
| `openai/gpt-5.2-codex` | An upgraded version of GPT-5.1-Codex optimized for software engineering and coding workflows | 400K | $1.75 | $14.00 | text, image | 2026-01-14 |
| `openai/gpt-5.2-chat` | GPT-5.2 Chat (AKA Instant) is the fast, lightweight member of the 5.2 family, optimized for low-latency chat while retaining strong general… | 128K | $1.75 | $14.00 | file, image, text | 2025-12-10 |
| `openai/gpt-5.2-pro` · :batch | OpenAI’s most advanced model, offering major improvements in agentic coding and long context performance over GPT-5 Pro | 400K | $21.00 | $168.00 | image, text, file | 2025-12-10 |
| `openai/gpt-5.2` · :batch | The latest frontier-grade model in the GPT-5 series, offering stronger agentic and long context perfomance compared to GPT-5.1 | 400K | $1.75 | $14.00 | file, image, text | 2025-12-10 |
| `openai/gpt-5.1-codex-max` | OpenAI’s latest agentic coding model, designed for long-running, high-context software development tasks | 400K | $1.25 | $10.00 | text, image | 2025-12-04 |
| `openai/gpt-5.1` · :batch | The latest frontier-grade model in the GPT-5 series, offering stronger general-purpose reasoning, improved instruction adherence, and a more natural… | 400K | $1.25 | $10.00 | image, text, file | 2025-11-13 |
| `openai/gpt-5.1-codex` | A specialized version of GPT-5.1 optimized for software engineering and coding workflows | 400K | $1.25 | $10.00 | text, image | 2025-11-13 |
| `openai/gpt-5.1-codex-mini` | A smaller and faster version of GPT-5.1-Codex | 400K | $0.25 | $2.00 | image, text | 2025-11-13 |
| `openai/gpt-oss-safeguard-20b` | A safety reasoning model from OpenAI built upon gpt-oss-20b | 131K | $0.075 | $0.30 | text | 2025-10-29 |
| `openai/gpt-5-pro` · :batch | OpenAI’s most advanced model, offering major improvements in reasoning, code quality, and user experience | 400K | $15.00 | $120.00 | image, text, file | 2025-10-06 |
| `openai/gpt-5` · :batch | OpenAI’s most advanced model, offering major improvements in reasoning, code quality, and user experience | 400K | $1.25 | $10.00 | text, image, file | 2025-08-07 |
| `openai/gpt-5-mini` · :batch | A compact version of GPT-5, designed to handle lighter-weight reasoning tasks | 400K | $0.25 | $2.00 | text, image, file | 2025-08-07 |
| `openai/gpt-5-nano` · :batch | GPT-5-Nano is the smallest and fastest variant in the GPT-5 system, optimized for developer tools, rapid interactions, and ultra-low latency… | 400K | $0.050 | $0.40 | text, image, file | 2025-08-07 |
| `openai/gpt-oss-120b` · :batch | An open-weight, 117B-parameter Mixture-of-Experts (MoE) language model from OpenAI designed for high-reasoning, agentic, and general-purpose… | 131K | $0.016 | $0.059 | text | 2025-08-05 |
| `openai/gpt-oss-20b` · :batch | An open-weight 21B parameter model released by OpenAI under the Apache 2.0 license | 131K | $0.030 | $0.13 | text | 2025-08-05 |
| `openai/o3-pro` | The o-series of models are trained with reinforcement learning to think before they answer and perform complex reasoning | 200K | $20.00 | $80.00 | text, file, image | 2025-06-10 |
| `openai/o3` · :batch | A well-rounded and powerful model across domains | 200K | $2.00 | $8.00 | image, text, file | 2025-04-16 |
| `openai/o4-mini:batch` | OpenAI o4-mini is a compact reasoning model in the o-series, optimized for fast, cost-efficient performance while retaining strong multimodal and… | 200K | $0.55 | $2.20 | image, text, file | 2025-04-16 |
| `openai/gpt-4.1` · :batch | A flagship large language model optimized for advanced instruction following, real-world software engineering, and long-context reasoning | 1.0M | $2.00 | $8.00 | image, text, file | 2025-04-14 |
| `openai/gpt-4.1-mini` · :batch | A mid-sized model delivering performance competitive with GPT-4o at substantially lower latency and cost | 1.0M | $0.40 | $1.60 | image, text, file | 2025-04-14 |
| `openai/gpt-4.1-nano` · :batch | For tasks that demand low latency, GPT‑4.1 nano is the fastest and cheapest model in the GPT-4.1 series | 1.0M | $0.100 | $0.40 | image, text, file | 2025-04-14 |
| `openai/o3-mini:batch` | OpenAI o3-mini is a cost-efficient language model optimized for STEM reasoning tasks, particularly excelling in science, mathematics, and coding | 200K | $0.55 | $2.20 | text, file | 2025-01-31 |
| `openai/gpt-4o-2024-11-20` | The 2024-11-20 version of GPT-4o offers a leveled-up creative writing ability with more natural, engaging, and tailored writing to improve relevance… | 128K | $2.50 | $10.00 | text, image, file | 2024-11-20 |
| `openai/gpt-4o-2024-08-06` | The 2024-08-06 version of GPT-4o offers improved performance in structured outputs, with the ability to supply a JSON schema in the respone_format | 128K | $2.50 | $10.00 | text, image, file | 2024-08-05 |
| `openai/gpt-4o-mini` · :batch | GPT-4o mini is OpenAI's newest model after GPT-4 Omni, supporting both text and image inputs with text outputs | 128K | $0.15 | $0.60 | text, image, file | 2024-07-17 |
| `openai/gpt-4o-mini-2024-07-18` | GPT-4o mini is OpenAI's newest model after GPT-4 Omni, supporting both text and image inputs with text outputs | 128K | $0.15 | $0.60 | text, image, file | 2024-07-17 |
| `openai/gpt-4o` · :batch | GPT-4o ("o" for "omni") is OpenAI's latest AI model, supporting both text and image inputs with text outputs | 128K | $2.50 | $10.00 | text, image, file | 2024-05-12 |
| `openai/gpt-4o-2024-05-13` | GPT-4o ("o" for "omni") is OpenAI's latest AI model, supporting both text and image inputs with text outputs | 128K | $5.00 | $15.00 | text, image, file | 2024-05-12 |
| `openai/gpt-4-turbo:batch` | The latest GPT-4 Turbo model with vision capabilities | 128K | $5.00 | $15.00 | text, image | 2024-04-08 |
| `openai/gpt-3.5-turbo-0613` | GPT-3.5 Turbo is OpenAI's fastest model | 4K | $1.00 | $2.00 | text | 2024-01-24 |
| `openai/gpt-3.5-turbo-16k` | This model offers four times the context length of gpt-3.5-turbo, allowing it to support approximately 20 pages of text in a single request at a… | 16K | $3.00 | $4.00 | text | 2023-08-27 |
| `openai/gpt-4` | OpenAI's flagship model, GPT-4 is a large-scale multimodal language model capable of solving difficult problems with greater accuracy than previous… | 8K | $30.00 | $60.00 | text | 2023-05-27 |

</details>

## google (21)

Google's Gemini: multimodal (text, images, audio, video in), very long context; Pro is the strong tier, Flash the fast one.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `google/gemini-4-argon` · :batch | Google's flagship reasoning model, built for complex software engineering, deep mathematical and scientific problem solving, and autonomous… | 1.0M | $2.00 | $10.00 | audio, file, image, text, video | 2026-10-13 |
| `google/gemini-3.8-flash` · :batch | Google's most intelligent Flash model with significant gains from 3.7 Flash across software engineering, agentic tasks, and multi-step reasoning | 1.0M | $0.83 | $4.12 | text, image, video, file, audio | 2026-09-02 |
| `google/gemini-3.5-flash-lite` · :batch | A high-efficiency model from Google with upgraded agentic capabilities | 1.0M | $0.30 | $2.50 | text, image, video, file, audio | 2026-07-21 |

<details><summary>Older google versions and lines (18)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `google/gemini-3.7-flash` · :batch | A multimodal model from Google for fast agentic workflows, coding, and complex multi-step reasoning | 1.0M | $0.75 | $3.75 | text, image, video, file, audio | 2026-08-13 |
| `google/gemini-3.6-flash` · :batch | A high-efficiency model from Google for coding, agentic workflows, and web and app development | 1.0M | $0.75 | $3.75 | text, image, video, file, audio | 2026-07-21 |
| `google/gemini-3.5-flash` · :batch | Google's high-efficiency multimodal model, bringing near-Pro level coding and reasoning at Flash-tier cost and speed | 1.0M | $1.50 | $9.00 | text, image, video, file, audio | 2026-05-19 |
| `google/gemini-3.1-flash-lite` · :batch | Google’s GA high-efficiency multimodal model optimized for low-latency, high-volume workloads | 1.0M | $0.25 | $1.50 | text, image, video, file, audio | 2026-05-07 |
| `google/gemma-4-26b-a4b-it` · :free | Gemma 4 26B A4B IT is an instruction-tuned Mixture-of-Experts (MoE) model from Google DeepMind | 262K | $0.076 | $0.26 | image, text, video | 2026-04-03 |
| `google/gemma-4-31b-it` · :free | Gemma 4 31B Instruct is Google DeepMind's 30.7B dense multimodal model supporting text and image input with text output | 262K | $0.20 | $0.40 | image, text, video | 2026-04-02 |
| `google/gemini-3.1-flash-lite-preview` | Google's high-efficiency model optimized for high-volume use cases | 1.0M | $0.25 | $1.50 | text, image, video, file, audio | 2026-03-02 |
| `google/gemini-3.1-pro-preview-customtools` | A variant of Gemini 3.1 Pro that improves tool selection behavior by preventing overuse of a general bash tool when more efficient third-party | 1.0M | $2.00 | $12.00 | text, audio, image, video, file | 2026-02-25 |
| `google/gemini-3.1-pro-preview` · :batch | Google’s frontier reasoning model, delivering enhanced software engineering performance, improved agentic reliability, and more efficient token usage… | 1.0M | $2.00 | $12.00 | audio, file, image, text, video | 2026-02-19 |
| `google/gemini-3-flash-preview` · :batch | A high speed, high value thinking model designed for agentic workflows, multi turn chat, and coding assistance | 1.0M | $0.50 | $3.00 | text, image, file, audio, video | 2025-12-17 |
| `google/gemini-2.5-flash-lite` | Gemini 2.5 Flash-Lite is a lightweight reasoning model in the Gemini 2.5 family, optimized for ultra-low latency and cost efficiency | 1.0M | $0.100 | $0.40 | text, image, file, audio, video | 2025-07-22 |
| `google/gemini-2.5-flash` | Google's state-of-the-art workhorse model, specifically designed for advanced reasoning, coding, mathematics, and scientific tasks | 1.0M | $0.30 | $2.50 | file, image, text, audio, video | 2025-06-17 |
| `google/gemini-2.5-pro` | Google’s state-of-the-art AI model designed for advanced reasoning, coding, mathematics, and scientific tasks | 1.0M | $1.25 | $10.00 | text, image, file, audio, video | 2025-06-17 |
| `google/gemini-2.5-pro-preview` | Gemini 2.5 Pro is Google’s state-of-the-art AI model designed for advanced reasoning, coding, mathematics, and scientific tasks | 1.0M | $1.25 | $10.00 | file, image, text, audio | 2025-06-05 |
| `google/gemma-3-4b-it` | Gemma 3 introduces multimodality, supporting vision-language input and text outputs | 131K | $0.050 | $0.100 | text, image | 2025-03-13 |
| `google/gemma-3-12b-it` | Gemma 3 introduces multimodality, supporting vision-language input and text outputs | 131K | $0.050 | $0.15 | text, image | 2025-03-13 |
| `google/gemma-3-27b-it` | Gemma 3 introduces multimodality, supporting vision-language input and text outputs | 131K | $0.080 | $0.35 | text, image | 2025-03-12 |
| `google/gemma-2-27b-it` | Google is an open model built from the same research and technology used to create the Gemini models | 8K | $0.65 | $0.65 | text | 2024-07-12 |

</details>

## x-ai (7)

xAI's Grok: general chat and reasoning, with live-web leaning.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `x-ai/grok-4.7` · :batch | SpaceXAI's flagship model for coding, agentic tasks, and knowledge work, succeeding Grok 4.6 | 500K | $2.00 | $6.00 | text, image, file | 2026-09-21 |
| `x-ai/grok-4.20-multi-agent` | A variant of SpaceXAI’s Grok 4.20 designed for collaborative, agent-based workflows | 2M | $1.25 | $2.50 | text, image, file | 2026-03-31 |
| `x-ai/grok-build-0.1` | SpaceXAI’s fast coding model trained specifically for agentic software engineering workflows | 256K | $1.00 | $2.00 | text, image, file | 2026-05-20 |

<details><summary>Older x-ai versions and lines (4)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `x-ai/grok-4.6` · :batch | A model from SpaceXAI with frontier performance on coding, knowledge work, and STEM | 500K | $2.00 | $6.00 | text, image, file | 2026-08-12 |
| `x-ai/grok-4.5` | A model from SpaceXAI with frontier performance on coding, knowledge work, and STEM | 500K | $2.00 | $6.00 | text, image, file | 2026-07-08 |
| `x-ai/grok-4.3` · :batch | A reasoning model from SpaceXAI | 1M | $1.25 | $2.50 | text, image, file | 2026-04-30 |
| `x-ai/grok-4.20` | A reasoning model from SpaceXAI with industry-leading speed and agentic tool calling capabilities | 2M | $1.25 | $2.50 | text, image, file | 2026-03-31 |

</details>

## meta (8)

Meta's newer models (Muse line).

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `meta/muse-wonder` | A frontier reasoning model from Meta, designed for complex personal and professional agentic tasks | 1.0M | $4.00 | $20.00 | text, image, video, file, audio | 2026-10-12 |
| `meta/muse-spark-open` | A coding-optimized open-weight model | 1.0M | $1.25 | $4.25 | text, image, video, file, audio | 2026-10-15 |
| `meta/muse-spark-1.3` | A multimodal reasoning model from Meta for long-running agentic, multi-agent, and coding workflows | 1.0M | $1.25 | $4.25 | text, image, video, file | 2026-09-02 |
| `meta/muse-glimmer-30b` | A dense, open-weight multimodal model from Meta Superintelligence Labs, distilled from Muse Spark and optimized for autonomous agents on consumer… | 131K | $0.30 | $1.20 | text, image | 2026-08-09 |
| `meta/muse-spark-1.3-contributor` | The cost-efficient contributor tier of Meta’s multimodal reasoning model for experimentation, learning, and early-stage agentic, multi-agent, and… | 1.0M | $0.100 | $0.20 | text, image, video, file | 2026-09-02 |

<details><summary>Older meta versions and lines (3)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `meta/muse-spark-1.2-contributor` | Muse Spark 1.2 contributor tier is a reasoning model from Meta designed for developers who want to start building at an even lower cost | 1.0M | $0.100 | $0.20 | text, image, video, file | 2026-08-21 |
| `meta/muse-spark-1.2` | A reasoning model from Meta, designed for complex agentic tasks | 1.0M | $1.25 | $4.25 | text, image, video, file | 2026-08-05 |
| `meta/muse-spark-1.1` | A multimodal reasoning model from Meta, built for agentic tasks | 1.0M | $1.25 | $4.25 | text, image, video, file | 2026-07-16 |

</details>

## mistralai (20)

Mistral (France): open-weight and commercial models; strong value for price, good at European languages.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `mistralai/mistral-medium-3-5` · :batch | A dense 128B instruction-following model from Mistral AI | 262K | $1.50 | $7.50 | text, image, file | 2026-04-30 |
| `mistralai/mistral-large-4-0` | A frontier multimodal (text and image input) model from Mistral AI built for reasoning, coding, and agentic workloads | 1.0M | $0.70 | $2.10 | text, image | 2026-10-06 |

<details><summary>Older mistralai versions and lines (18)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `mistralai/mistral-small-2603` | The next major release in the Mistral Small family, unifying the capabilities of several flagship Mistral models into a single system | 262K | $0.15 | $0.60 | text, image | 2026-03-16 |
| `mistralai/devstral-2512` | Devstral 2 is a state-of-the-art open-source model by Mistral AI specializing in agentic coding | 262K | $0.40 | $2.00 | text, file | 2025-12-09 |
| `mistralai/ministral-14b-2512` | The largest model in the Ministral 3 family, Ministral 3 14B offers frontier capabilities and performance comparable to its larger Mistral Small 3.2… | 262K | $0.20 | $0.20 | text, image | 2025-12-02 |
| `mistralai/ministral-8b-2512` | A balanced model in the Ministral 3 family, Ministral 3 8B is a powerful, efficient tiny language model with vision capabilities | 262K | $0.15 | $0.15 | text, image | 2025-12-02 |
| `mistralai/ministral-3b-2512` | The smallest model in the Ministral 3 family, Ministral 3 3B is a powerful, efficient tiny language model with vision capabilities | 131K | $0.100 | $0.100 | text, image | 2025-12-02 |
| `mistralai/mistral-large-2512` | Mistral’s most capable model to date, featuring a sparse mixture-of-experts architecture with 41B active parameters (675B total), and released under… | 262K | $0.50 | $1.50 | text, image, file | 2025-12-01 |
| `mistralai/voxtral-small-24b-2507` | Voxtral Small is an enhancement of Mistral Small 3, incorporating state-of-the-art audio input capabilities while retaining best-in-class text… | 33K | $0.100 | $0.30 | text, audio, file | 2025-10-30 |
| `mistralai/mistral-medium-3.1` | An updated version of Mistral Medium 3, which is a high-performance enterprise-grade language model designed to deliver frontier-level capabilities… | 131K | $0.40 | $2.00 | text, image, file | 2025-08-13 |
| `mistralai/codestral-2508` | Mistral's cutting-edge language model for coding released end of July 2025 | 256K | $0.30 | $0.90 | text, file | 2025-08-01 |
| `mistralai/mistral-small-3.2-24b-instruct` | Mistral-Small-3.2-24B-Instruct-2506 is an updated 24B parameter model from Mistral optimized for instruction following, repetition reduction, and… | 256K | $0.075 | $0.20 | image, text | 2025-06-20 |
| `mistralai/mistral-medium-3` | A high-performance enterprise-grade language model designed to deliver frontier-level capabilities at significantly reduced operational cost | 131K | $0.40 | $2.00 | text, image, file | 2025-05-07 |
| `mistralai/mistral-small-3.1-24b-instruct` | Mistral Small 3.1 24B Instruct is an upgraded variant of Mistral Small 3 (2501), featuring 24 billion parameters with advanced multimodal capabilities | 128K | $0.35 | $0.55 | text, image | 2025-03-17 |
| `mistralai/mistral-saba` | Mistral Saba is a 24B-parameter language model specifically designed for the Middle East and South Asia, delivering accurate and contextually… | 33K | $0.20 | $0.60 | text, file | 2025-02-17 |
| `mistralai/mistral-small-24b-instruct-2501` | A 24B-parameter language model optimized for low-latency performance across common AI tasks | 33K | $0.050 | $0.080 | text | 2025-01-30 |
| `mistralai/mistral-large-2407` | This is Mistral AI's flagship model, Mistral Large 2 (version mistral-large-2407) | 131K | $2.00 | $6.00 | text, file | 2024-11-18 |
| `mistralai/mistral-nemo` | A 12B parameter model with a 128k token context length built by Mistral in collaboration with NVIDIA | 131K | $0.029 | $0.030 | text | 2024-07-18 |
| `mistralai/mixtral-8x22b-instruct` | Mistral's official instruct fine-tuned version of Mixtral 8x22B | 66K | $2.00 | $6.00 | text, file | 2024-04-16 |
| `mistralai/mistral-large` | This is Mistral AI's flagship model, Mistral Large 2 (version `mistral-large-2407`) | 128K | $2.00 | $6.00 | text, file | 2024-02-25 |

</details>

## deepseek (14)

DeepSeek (China): open-weight, very cheap for their strength, good at reasoning and coding.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `deepseek/deepseek-v4-pro-0813` | A large-scale mixture-of-experts model from DeepSeek | 1.0M | $0.10 | $4.11 | text | 2026-08-12 |
| `deepseek/deepseek-v4-flash-vision-exp` | An experimental vision-enabled version of DeepSeek V4 Flash 0731 from DeepSeek, adding image understanding while matching the base model on text… | 1.0M | $0.44 | $1.32 | text, image | 2026-08-21 |
| `deepseek/deepseek-v4.1-flash-1017` | The October 17, 2026 snapshot of DeepSeek V4.1 Flash, served by DeepSeek under the `deepseek-flash` API model name | 1.0M | $0.30 | $1.20 | text, image | 2026-10-17 |

<details><summary>Older deepseek versions and lines (11)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `deepseek/deepseek-v4.1-flash` · :batch | A sparse mixture-of-experts model from DeepSeek, and the first built on the company's Causal Encoder-Decoder (CED) architecture | 1.0M | $0.045 | $0.50 | text, image | 2026-09-10 |
| `deepseek/deepseek-v4-flash-0731` | A sparse mixture-of-experts model from DeepSeek, with 13B active parameters out of 284B total | 1.0M | $0.059 | $0.30 | text | 2026-07-31 |
| `deepseek/deepseek-v4-pro` | DeepSeek V4 Pro is a large-scale Mixture-of-Experts model from DeepSeek with 1.6T total parameters and 49B activated parameters, supporting a… | 1.0M | $0.96 | $1.91 | text | 2026-04-23 |
| `deepseek/deepseek-v4-flash` | DeepSeek V4 Flash is an efficiency-optimized Mixture-of-Experts model from DeepSeek with 284B total parameters and 13B activated parameters,… | 1.0M | $0.091 | $0.18 | text | 2026-04-23 |
| `deepseek/deepseek-v3.2` | DeepSeek-V3.2 is a large language model designed to harmonize high computational efficiency with strong reasoning and agentic tool-use performance | 164K | $0.26 | $0.38 | text | 2025-12-01 |
| `deepseek/deepseek-v3.2-exp` | DeepSeek-V3.2-Exp is an experimental large language model released by DeepSeek as an intermediate step between V3.1 and future architectures | 164K | $0.27 | $0.41 | text | 2025-09-29 |
| `deepseek/deepseek-v3.1-terminus` | DeepSeek-V3.1 Terminus is an update to DeepSeek V3.1 that maintains the model's original capabilities while addressing issues reported by users,… | 164K | $0.27 | $1.00 | text | 2025-09-22 |
| `deepseek/deepseek-chat-v3.1` | DeepSeek-V3.1 is a large hybrid reasoning model (671B parameters, 37B active) that supports both thinking and non-thinking modes via prompt templates | 164K | $0.25 | $0.95 | text | 2025-08-21 |
| `deepseek/deepseek-r1-0528` | May 28th update to the original DeepSeek R1 Performance on par with OpenAI o1, but open-sourced and with fully open reasoning tokens | 164K | $0.50 | $2.15 | text | 2025-05-28 |
| `deepseek/deepseek-chat-v3-0324` | DeepSeek V3, a 685B-parameter, mixture-of-experts model, is the latest iteration of the flagship chat model family from the DeepSeek team | 164K | $0.29 | $1.14 | text | 2025-03-24 |
| `deepseek/deepseek-chat` | DeepSeek-V3 is the latest model from the DeepSeek team, building upon the instruction following and coding abilities of the previous versions | 164K | $0.26 | $1.03 | text | 2024-12-26 |

</details>

## qwen (41)

Alibaba's Qwen: open-weight family, strong multilingual and coding models in many sizes.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `qwen/qwen3.8-max-prime` | A higher-throughput variant of Qwen3.8 Max from Alibaba's Qwen team, served as a separate SKU at a higher price point | 1M | $4.00 | $12.00 | text, image, video | 2026-09-23 |
| `qwen/qwen3.8-max-0902` | Qwen3.8 Max 0902 is an updated snapshot of Qwen3.8 Max from Alibaba's Qwen team | 1M | $2.00 | $6.00 | text, image, video | 2026-09-03 |
| `qwen/qwen3.8-2.4t-a95b` | An open-weight sparse mixture-of-experts model from Qwen and the open-weight variant of Qwen3.8 Max, with 95 billion active parameters out of 2.4… | 1.0M | $2.00 | $6.00 | text | 2026-08-12 |
| `qwen/qwen3.8-27b` | An open-weight dense vision-language model from Qwen | 1.0M | $0.15 | $1.88 | text, image, video | 2026-08-14 |
| `qwen/qwen3.7-plus` · :batch | Qwen3.7-Plus is a cost-effective model in Alibaba's Qwen3.7 series | 1M | $0.32 | $1.28 | text, image | 2026-06-03 |
| `qwen/qwen3.6-35b-a3b` | Qwen3.6-35B-A3B is an open-weight multimodal model from Alibaba Cloud with 35 billion total parameters and 3 billion active parameters per token | 262K | $0.100 | $0.90 | text, image, video | 2026-04-26 |
| `qwen/qwen3.8-omni-flash` | An omni-modal reasoning model from Alibaba, the first Qwen model built around agentic capabilities with native audio-video understanding | 1M | $0.15 | $0.47 | text, image, audio, video | 2026-09-20 |
| `qwen/qwen3.8-flash` | A multimodal reasoning model from Alibaba | 1M | $0.15 | $0.47 | text, image, video | 2026-08-26 |

<details><summary>Older qwen versions and lines (33)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `qwen/qwen3.7-flash` | A vision-language reasoning model from Alibaba | 1M | $0.030 | $0.13 | text, image, video | 2026-07-27 |
| `qwen/qwen3.7-max` | Qwen3.7-Max is the flagship model in Alibaba's Qwen3.7 series | 1M | $1.48 | $4.42 | text | 2026-05-21 |
| `qwen/qwen3.5-plus-20260420` | Qwen3.5 Plus (April 2026) is a large-scale multimodal language model from Alibaba | 1M | $0.30 | $1.80 | text, image, video | 2026-04-26 |
| `qwen/qwen3.6-flash` | A fast, efficient language model from Alibaba's Qwen 3.6 series | 1M | $0.19 | $1.12 | text, image, video | 2026-04-26 |
| `qwen/qwen3.6-27b` | A dense 27-billion-parameter language model from the Qwen Team at Alibaba, released in April 2026 | 262K | $0.45 | $2.70 | text, image, video | 2026-04-26 |
| `qwen/qwen3.6-plus` | Qwen 3.6 Plus builds on a hybrid architecture that combines efficient linear attention with sparse mixture-of-experts routing, enabling strong… | 1M | $0.33 | $1.95 | text, image, video | 2026-04-02 |
| `qwen/qwen3.5-9b` | A multimodal foundation model from the Qwen3.5 family, designed to deliver strong reasoning, coding, and visual understanding in an efficient… | 262K | $0.17 | $0.25 | text, image, video | 2026-03-10 |
| `qwen/qwen3.5-35b-a3b` | The Qwen3.5 Series 35B-A3B is a native vision-language model designed with a hybrid architecture that integrates linear attention mechanisms and a… | 262K | $0.14 | $1.00 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-27b` | The Qwen3.5 27B native vision-language Dense model incorporates a linear attention mechanism, delivering fast response times while balancing… | 262K | $0.26 | $2.60 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-122b-a10b` | The Qwen3.5 122B-A10B native vision-language model is built on a hybrid architecture that integrates a linear attention mechanism with a sparse… | 262K | $0.26 | $2.08 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-flash-02-23` | The Qwen3.5 native vision-language Flash models are built on a hybrid architecture that integrates a linear attention mechanism with a sparse… | 1M | $0.065 | $0.26 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-plus-02-15` | The Qwen3.5 native vision-language series Plus models are built on a hybrid architecture that integrates linear attention mechanisms with sparse… | 1M | $0.26 | $1.56 | text, image, video | 2026-02-16 |
| `qwen/qwen3.5-397b-a17b` | The Qwen3.5 series 397B-A17B native vision-language model is built on a hybrid architecture that integrates a linear attention mechanism with a… | 262K | $0.50 | $3.60 | text, image, video | 2026-02-16 |
| `qwen/qwen3-coder-next` | Qwen3-Coder-Next is an open-weight causal language model optimized for coding agents and local development workflows | 262K | $0.12 | $0.80 | text | 2026-02-03 |
| `qwen/qwen3-vl-8b-instruct` | Qwen3-VL-8B-Instruct is a multimodal vision-language model from the Qwen3-VL series, built for high-fidelity understanding and reasoning across text,… | 262K | $0.25 | $0.75 | image, text | 2025-10-14 |
| `qwen/qwen3-vl-30b-a3b-thinking` | Qwen3-VL-30B-A3B-Thinking is a multimodal model that unifies strong text generation with visual understanding for images and videos | 262K | $0.29 | $1.00 | text, image | 2025-10-06 |
| `qwen/qwen3-vl-30b-a3b-instruct` | Qwen3-VL-30B-A3B-Instruct is a multimodal model that unifies strong text generation with visual understanding for images and videos | 262K | $0.15 | $0.60 | text, image | 2025-10-06 |
| `qwen/qwen3-vl-235b-a22b-instruct` | Qwen3-VL-235B-A22B Instruct is an open-weight multimodal model that unifies strong text generation with visual understanding across images and video | 262K | $0.21 | $1.90 | text, image | 2025-09-23 |
| `qwen/qwen3-coder-flash` | Alibaba's fast and cost efficient version of their proprietary Qwen3 Coder Plus | 1M | $0.20 | $0.97 | text | 2025-09-17 |
| `qwen/qwen3-next-80b-a3b-instruct` | Qwen3-Next-80B-A3B-Instruct is an instruction-tuned chat model in the Qwen3-Next series optimized for fast, stable responses without “thinking” traces | 262K | $0.090 | $1.10 | text | 2025-09-11 |
| `qwen/qwen3-coder-30b-a3b-instruct` | Qwen3-Coder-30B-A3B-Instruct is a 30.5B parameter Mixture-of-Experts (MoE) model with 128 experts (8 active per forward pass), designed for advanced… | 262K | $0.15 | $0.60 | text | 2025-07-31 |
| `qwen/qwen3-30b-a3b-instruct-2507` | Qwen3-30B-A3B-Instruct-2507 is a 30.5B-parameter mixture-of-experts language model from Qwen, with 3.3B active parameters per inference | 262K | $0.100 | $0.30 | text | 2025-07-29 |
| `qwen/qwen3-235b-a22b-thinking-2507` | Qwen3-235B-A22B-Thinking-2507 is a high-performance, open-weight Mixture-of-Experts (MoE) language model optimized for complex reasoning tasks | 128K | $0.45 | $3.50 | text | 2025-07-25 |
| `qwen/qwen3-coder` | Qwen3-Coder-480B-A35B-Instruct is a Mixture-of-Experts (MoE) code generation model developed by the Qwen team | 262K | $0.30 | $1.00 | text | 2025-07-22 |
| `qwen/qwen3-235b-a22b-2507` | Qwen3-235B-A22B-Instruct-2507 is a multilingual, instruction-tuned mixture-of-experts language model based on the Qwen3-235B architecture, with 22B… | 262K | $0.14 | $0.80 | text | 2025-07-21 |
| `qwen/qwen3-30b-a3b` | Qwen3, the latest generation in the Qwen large language model series, features both dense and mixture-of-experts (MoE) architectures to excel in… | 41K | $0.12 | $0.50 | text | 2025-04-28 |
| `qwen/qwen3-14b` | Qwen3-14B is a dense 14.8B parameter causal language model from the Qwen3 series, designed for both complex reasoning and efficient dialogue | 41K | $0.12 | $0.24 | text | 2025-04-28 |
| `qwen/qwen3-32b` | Qwen3-32B is a dense 32.8B parameter causal language model from the Qwen3 series, optimized for both complex reasoning and efficient dialogue | 131K | $0.080 | $0.28 | text | 2025-04-28 |
| `qwen/qwen2.5-vl-72b-instruct` | Qwen2.5-VL is proficient in recognizing common objects such as flowers, birds, fish, and insects | 128K | $0.80 | $1.00 | text, image | 2025-02-01 |
| `qwen/qwen-plus` | Based on the Qwen2.5 foundation model, is a 131K context model with a balanced performance, speed, and cost combination | 1M | $0.26 | $0.78 | text | 2025-02-01 |
| `qwen/qwen-2.5-coder-32b-instruct` | Qwen2.5-Coder is the latest series of Code-Specific Qwen large language models (formerly known as CodeQwen) | 33K | $0.66 | $1.00 | text | 2024-11-11 |
| `qwen/qwen-2.5-7b-instruct` | Qwen2.5 7B is the latest series of Qwen large language models | 33K | $0.100 | $0.20 | text | 2024-10-15 |
| `qwen/qwen-2.5-72b-instruct` | Qwen2.5 72B is the latest series of Qwen large language models | 33K | $0.36 | $0.40 | text | 2024-09-18 |

</details>

## moonshotai (8)

Moonshot's Kimi: long context and agentic tool use.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `moonshotai/kimi-k3` · :batch | A 2.8T parameter open-weight multimodal reasoning model from Moonshot AI | 1.0M | $0.19 | $16.00 | text, image, video | 2026-07-16 |
| `moonshotai/kimi-k3.1-pro` | Moonshot AI's flagship open-weight multimodal reasoning model and a post-trained successor to Kimi K3, with a 1M-token context window | 1.0M | $2.00 | $10.00 | text, image, video | 2026-10-23 |
| `moonshotai/kimi-k2.7-code` | MoonshotAI: Kimi K2.7 Code is a coding-focused model in Moonshot AI's Kimi K2 family, built to complete end-to-end programming tasks reliably over… | 262K | $0.60 | $3.50 | text, image | 2026-06-12 |

<details><summary>Older moonshotai versions and lines (5)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `moonshotai/kimi-k2.6` | Moonshot AI's next-generation multimodal model, designed for long-horizon coding, coding-driven UI/UX generation, and multi-agent orchestration | 262K | $0.65 | $3.41 | text, image | 2026-04-20 |
| `moonshotai/kimi-k2.5` | Moonshot AI's native multimodal model, delivering state-of-the-art visual coding capability and a self-directed agent swarm paradigm | 262K | $0.50 | $2.50 | text, image | 2026-01-26 |
| `moonshotai/kimi-k2-thinking` | Moonshot AI’s most advanced open reasoning model to date, extending the K2 series into agentic, long-horizon reasoning | 262K | $0.60 | $2.50 | text | 2025-11-06 |
| `moonshotai/kimi-k2-0905` | The September update of Kimi K2 0711 | 262K | $0.60 | $2.50 | text | 2025-09-04 |
| `moonshotai/kimi-k2` | Kimi K2 Instruct is a large-scale Mixture-of-Experts (MoE) language model developed by Moonshot AI, featuring 1 trillion total parameters with 32… | 131K | $0.57 | $2.30 | text | 2025-07-11 |

</details>

## z-ai (17)

Zhipu's GLM: Chinese open-weight family, good at agents and coding; FlashX variants are cheap.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `z-ai/glm-5.3-prime` | GLM-5.3-Prime is the high-speed variant of Z.ai's GLM-5.3, inheriting its full capabilities while delivering 1.5–2× the output throughput through… | 1M | $2.80 | $8.80 | text | 2026-09-23 |
| `z-ai/glm-5.3` · :batch | GLM-5.3 is a large-scale reasoning model from Z.ai, built for complex software engineering and long-horizon agent tasks | 1.0M | $0.070 | $1.40 | text | 2026-08-18 |
| `z-ai/glm-5.3-flashx-legal` | GLM-5.3-FlashX-Legal (GLM-Legal) is Z.ai's legal-domain variant of GLM-5.3-FlashX, a native multimodal model that accepts text, image, and video… | 1.0M | $0.37 | $1.25 | text, image, video | 2026-11-02 |
| `z-ai/glm-5.3-flashx` | GLM-5.3-FlashX is the high-speed variant of Z.ai's GLM-5.3-Flash, a native multimodal model delivering inference speeds of up to 200 tokens/s | 1.0M | $0.37 | $1.25 | text, image, video | 2026-09-18 |
| `z-ai/glm-5.3-flash` · :batch | GLM-5.3-Flash is a native multimodal model from Z.ai | 1.0M | $0.15 | $0.50 | text, image, video | 2026-08-26 |

<details><summary>Older z-ai versions and lines (12)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `z-ai/glm-5.2` | A large-scale reasoning model from Z.ai | 1.0M | $0.006 | $6.80 | text | 2026-06-16 |
| `z-ai/glm-5.1` | GLM-5.1 delivers a major leap in coding capability, with particularly significant gains in handling long-horizon tasks | 205K | $1.40 | $4.40 | text | 2026-04-07 |
| `z-ai/glm-5v-turbo` | GLM-5V-Turbo is Z.ai’s first native multimodal agent foundation model, built for vision-based coding and agent-driven tasks | 203K | $1.20 | $4.00 | image, text, video | 2026-04-01 |
| `z-ai/glm-5-turbo` | GLM-5 Turbo is a new model from Z.ai designed for fast inference and strong performance in agent-driven environments such as OpenClaw scenarios | 203K | $1.20 | $4.00 | text | 2026-03-15 |
| `z-ai/glm-5` | GLM-5 is Z.ai’s flagship open-source foundation model engineered for complex systems design and long-horizon agent workflows | 205K | $1.00 | $3.20 | text | 2026-02-11 |
| `z-ai/glm-4.7-flash` | As a 30B-class SOTA model, GLM-4.7-Flash offers a new option that balances performance and efficiency | 200K | $0.061 | $0.40 | text | 2026-01-19 |
| `z-ai/glm-4.7` | GLM-4.7 is Z.ai’s latest flagship model, featuring upgrades in two key areas: enhanced programming capabilities and more stable multi-step… | 205K | $0.60 | $2.20 | text | 2025-12-21 |
| `z-ai/glm-4.6v` | GLM-4.6V is a large multimodal model designed for high-fidelity visual understanding and long-context reasoning across images, documents, and mixed… | 131K | $0.30 | $0.90 | image, text, video | 2025-12-08 |
| `z-ai/glm-4.6` | Compared with GLM-4.5, this generation brings several key improvements: Longer context window: The context window has been expanded from 128K to 200K… | 205K | $0.50 | $2.00 | text | 2025-09-30 |
| `z-ai/glm-4.5v` | GLM-4.5V is a vision-language foundation model for multimodal agent applications | 66K | $0.60 | $1.80 | text, image | 2025-08-11 |
| `z-ai/glm-4.5` | GLM-4.5 is our latest flagship foundation model, purpose-built for agent-based applications | 131K | $0.60 | $2.20 | text | 2025-07-25 |
| `z-ai/glm-4.5-air` | GLM-4.5-Air is the lightweight variant of our latest flagship model family, also purpose-built for agent-centric applications | 131K | $0.13 | $0.85 | text | 2025-07-25 |

</details>

## minimax (5)

MiniMax: long-context, cheap general models.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `minimax/minimax-m3` | MiniMax-M3 is a multimodal foundation model from MiniMax | 1.0M | $0.30 | $1.20 | text, image, video | 2026-05-31 |
| `minimax/minimax-m3.1-flash` | MiniMax-M3.1-Flash is a multimodal model from MiniMax | 1.0M | $0.15 | $0.60 | text, image, video | 2026-10-18 |

<details><summary>Older minimax versions and lines (3)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `minimax/minimax-m2.7` | MiniMax-M2.7 is a next-generation large language model designed for autonomous, real-world productivity and continuous improvement | 205K | $0.21 | $0.84 | text | 2026-03-18 |
| `minimax/minimax-m2.5` | MiniMax-M2.5 is a SOTA large language model designed for real-world productivity | 205K | $0.27 | $0.95 | text | 2026-02-12 |
| `minimax/minimax-m1` | MiniMax-M1 is a large-scale, open-weight reasoning model designed for extended context and high-efficiency inference | 1M | $0.55 | $2.20 | text | 2025-06-17 |

</details>

## nvidia (8)

NVIDIA's Nemotron: tuned open models, often free to try.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `nvidia/nemotron-3-ultra-550b-a55b` · :free | NVIDIA Nemotron 3 Ultra is an open frontier-reasoning and orchestration model from NVIDIA, with 55B active parameters out of 550B total (MoE) | 262K | $0.60 | $2.40 | text | 2026-06-04 |
| `nvidia/nemotron-3.5-super-vl` · :free | NVIDIA Nemotron 3.5 Super VL is an open multimodal mixture-of-experts model from NVIDIA, with 12B active parameters out of 120B total | 1.0M | $0.15 | $0.50 | text, image, video | 2026-10-15 |
| `nvidia/nemotron-3.5-lightning` · :free | NVIDIA Nemotron 3.5 Lightning is an open mixture-of-experts model from NVIDIA, with 3B active parameters out of 30B total | 262K | $0.070 | $0.20 | text | 2026-08-11 |
| `nvidia/nemotron-3.5-content-safety` · :free | NVIDIA Nemotron 3.5 Content Safety is a compact 4B-parameter multimodal guardrail model from NVIDIA, fine-tuned from Google Gemma-3-4B | 131K | $0.20 | $0.20 | text, image | 2026-06-04 |
| `nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free` | NVIDIA Nemotron™ 3 Nano Omni is a 30B-A3B open multimodal model designed to function as a perception and context sub-agent in enterprise agent systems | 256K | free | free | text, audio, image, video | 2026-04-28 |
| `nvidia/switchyard` | An open-source model routing system that switches between multiple models to optimize the cost of requests | 1M | $-1000000.000 | $-1000000.000 | text | 2026-09-21 |

<details><summary>Older nvidia versions and lines (2)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `nvidia/nemotron-3-super-120b-a12b` | NVIDIA Nemotron 3 Super is a 120B-parameter open hybrid MoE model, activating just 12B parameters for maximum compute efficiency and accuracy in… | 1.0M | $0.30 | $2.40 | text | 2026-03-11 |
| `nvidia/nemotron-3-nano-30b-a3b` | NVIDIA Nemotron 3 Nano 30B A3B is a small language MoE model with highest compute efficiency and accuracy for developers to build specialized agentic… | 262K | $0.060 | $0.24 | text | 2025-12-14 |

</details>

## amazon (5)

Amazon's Nova: cheap general and multimodal models.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `amazon/nova-2-lite-v1` | A fast, cost-effective reasoning model for everyday workloads that can process text, images, and videos to generate text | 1M | $0.30 | $2.50 | text, image, video, file | 2025-12-02 |

<details><summary>Older amazon versions and lines (3)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `amazon/nova-lite-v1` | Amazon Nova Lite 1.0 is a very low-cost multimodal model from Amazon that focused on fast processing of image, video, and text inputs to generate… | 300K | $0.060 | $0.24 | text, image | 2024-12-05 |
| `amazon/nova-micro-v1` | Amazon Nova Micro 1.0 is a text-only model that delivers the lowest latency responses in the Amazon Nova family of models at a very low cost | 128K | $0.035 | $0.14 | text | 2024-12-05 |
| `amazon/nova-pro-v1` | Amazon Nova Pro 1.0 is a capable multimodal model from Amazon focused on providing a combination of accuracy, speed, and cost for a wide range of… | 300K | $0.80 | $3.20 | text, image | 2024-12-05 |

</details>

## cohere (6)

Cohere's Command: enterprise chat and retrieval (RAG).

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `cohere/command-a-plus` | Cohere's flagship model for enterprise agentic workflows | 192K | $0.30 | $1.50 | text, image | 2026-09-22 |
| `cohere/north-mini-code:free` | North Mini Code is Cohere's first agentic coding model and the debut of its North family | 256K | free | free | text | 2026-06-17 |

<details><summary>Older cohere versions and lines (4)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `cohere/command-a` | An open-weights 111B parameter model with a 256k context window focused on delivering great performance across agentic, multilingual, and coding use… | 256K | $2.50 | $10.00 | text | 2025-03-13 |
| `cohere/command-r7b-12-2024` | A small, fast update of the Command R+ model, delivered in December 2024 | 128K | $0.037 | $0.15 | text | 2024-12-14 |
| `cohere/command-r-08-2024` | Command-r-08-2024 is an update of the Command R with improved performance for multilingual retrieval-augmented generation (RAG) and tool use | 128K | $0.15 | $0.60 | text | 2024-08-29 |
| `cohere/command-r-plus-08-2024` | Command-r-plus-08-2024 is an update of the Command R+ with roughly 50% higher throughput and 25% lower latencies as compared to the previous Command… | 128K | $2.50 | $10.00 | text | 2024-08-29 |

</details>

## aion-labs (6)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `aion-labs/aion-3.5` | A multi-model roleplaying and storytelling system from AionLabs, built on the GLM family of models | 262K | $3.00 | $6.00 | text | 2026-09-23 |
| `aion-labs/aion-3.5-mini` | A multi-model roleplaying and storytelling system from AionLabs, built on the GLM family of models | 262K | $0.70 | $1.40 | text | 2026-09-23 |

<details><summary>Older aion-labs versions and lines (4)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `aion-labs/aion-3.0-mini` | Aion-3.0 Mini is a multi-model roleplaying and storytelling system from AionLabs, built on the DeepSeek family of models | 131K | $0.70 | $1.40 | text | 2026-07-07 |
| `aion-labs/aion-3.0` | A multi-model roleplaying and storytelling system from AionLabs, built on the GLM family of models | 131K | $3.00 | $6.00 | text | 2026-07-07 |
| `aion-labs/aion-2.0` | A variant of DeepSeek V3.2 optimized for immersive roleplaying and storytelling | 131K | $0.80 | $1.60 | text | 2026-02-23 |
| `aion-labs/aion-rp-llama-3.1-8b` | Aion-RP-Llama-3.1-8B ranks the highest in the character evaluation portion of the RPBench-Auto benchmark, a roleplaying-specific variant of… | 33K | $0.80 | $1.60 | text | 2025-02-04 |

</details>

## bytedance-seed (6)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `bytedance-seed/seed-2.0-code` | Seed 2.0 Code is a model from ByteDance Seed optimized for agentic coding | 262K | $0.50 | $3.00 | text, image, video | 2026-08-12 |
| `bytedance-seed/seed-2-1-turbo` | A multimodal model from ByteDance Seed for coding and long-horizon agent workflows | 262K | $0.50 | $2.50 | text, image, video | 2026-08-12 |
| `bytedance-seed/seed-2.0-lite` | A versatile, cost‑efficient enterprise workhorse that delivers strong multimodal and agent capabilities while offering noticeably lower latency,… | 262K | $0.25 | $2.00 | text, image, video | 2026-03-10 |
| `bytedance-seed/seed-2.0-mini` | Seed-2.0-mini targets latency-sensitive, high-concurrency, and cost-sensitive scenarios, emphasizing fast response and flexible inference deployment | 262K | $0.100 | $0.40 | text, image, video | 2026-02-26 |

<details><summary>Older bytedance-seed versions and lines (2)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `bytedance-seed/seed-1.6-flash` | An ultra-fast multimodal deep thinking model by ByteDance Seed, supporting both text and visual understanding | 262K | $0.075 | $0.30 | image, text, video | 2025-12-23 |
| `bytedance-seed/seed-1.6` | A general-purpose model released by the ByteDance Seed team | 262K | $0.25 | $2.00 | image, text, video | 2025-12-23 |

</details>

## dots-studio (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `dots-studio/dots-3-note-preview:free` | Dots3-Note Preview is an open-weight mixture-of-experts model from Dots Studio, with 16B active parameters out of 280B total | 512K | free | free | text, image | 2026-08-14 |

## elorian (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `elorian/glimpse` | A multimodal visual reasoning model from Elorian, built for visual understanding and complex visual reasoning in the physical world | 262K | $0.60 | $3.00 | text, image, video | 2026-10-16 |

## fireworks (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `fireworks/ember-1` | A specialized reasoning model from Fireworks Research, built on Kimi K3 | 1.0M | $3.00 | $15.00 | text, image | 2026-09-23 |

## ibm-granite (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `ibm-granite/granite-4.2-8b` | A dense reasoning model from IBM | 131K | $0.060 | $0.25 | text | 2026-08-31 |

## inception (3)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `inception/mercury-3` | A diffusion-based language model from Inception, and the lab's most capable model to date | 1M | $0.100 | $0.35 | text | 2026-10-16 |
| `inception/mercury-2.5` | The fastest reasoning LLM, and the latest diffusion LLM (dLLM) from Inception | 260K | $0.040 | $0.15 | text | 2026-09-08 |
| `inception/mercury-2` | An extremely fast reasoning LLM, and the first reasoning diffusion LLM (dLLM) | 128K | $0.25 | $0.75 | text | 2026-03-04 |

## inclusionai (4)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `inclusionai/ling-3.1-flash` | A hybrid reasoning mixture-of-experts model from inclusionAI, with 25B active parameters out of 560B total | 1M | $0.075 | $0.22 | text | 2026-10-02 |
| `inclusionai/ling-3.0-flash-vl` | Ling 3.0 Flash VL builds on Ling 3.0 Flash (124B total / 5.5B active MoE from InclusionAI), further strengthening its language capabilities while… | 131K | $0.060 | $0.18 | text, image, video | 2026-09-10 |
| `inclusionai/ling-3.0-tiny` | A mixture-of-experts model from InclusionAI, with 1.3B active parameters out of 7.9B total | 262K | $0.015 | $0.045 | text | 2026-08-06 |

<details><summary>Older inclusionai versions and lines (1)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `inclusionai/ling-3.0-flash` | *Ling-3.0-flash* is a *124B-parameter Mixture-of-Experts (MoE) model*, with approximately *5.1B parameters activated per token* | 262K | $0.021 | $0.063 | text | 2026-07-23 |

</details>

## inference-net (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `inference-net/schematron-v2-small` | A 3B-parameter HTML-to-JSON extraction model from Inference.net | 128K | $0.050 | $0.23 | text | 2026-09-11 |
| `inference-net/schematron-v2-turbo` | A 3B-parameter HTML-to-JSON extraction model from Inference.net | 128K | $0.030 | $0.15 | text | 2026-09-11 |

## liquid (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `liquid/lfm-2.5-24b-a2b` | A mixture-of-experts reasoning model from Liquid AI with 24B total and 2.45B active parameters | 128K | $0.050 | $0.70 | text | 2026-10-13 |
| `liquid/lfm-2.5-2.6b` · :free | A compact reasoning model from Liquid AI | 128K | $0.35 | $0.50 | text | 2026-08-11 |

## meituan (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `meituan/longcat-2.0` | A sparse mixture-of-experts language model from Meituan, with 48B active parameters out of 1.6T total | 1.0M | $0.30 | $1.20 | text | 2026-07-20 |

## moondream (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `moondream/moondream3.1-9b-a2b` | A vision-language model from Moondream, the successor to Moondream 3 Preview, with 9B total parameters and 2B active (mixture-of-experts) | 32K | $0.30 | $2.50 | text, image | 2026-10-15 |
| `moondream/moondream3-preview` | A vision-language model from Moondream with 9B total parameters and 2B active (mixture-of-experts) | 32K | $0.30 | $2.50 | text, image | 2026-10-15 |

## nex-agi (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `nex-agi/nex-n2.5-pro` | Nex-N2.5 is an agentic model built to turn goals into working, verified outcomes | 262K | $0.075 | $0.25 | text, image | 2026-09-08 |
| `nex-agi/nex-n2.5-mini` | Nex-N2.5 is an agentic model built to turn goals into working, verified outcomes | 262K | $0.025 | $0.100 | text, image | 2026-09-08 |

## openrouter (4)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `openrouter/fusion` | Fusion turns your prompt into a small multi-model deliberation | 1M | $-1000000.000 | $-1000000.000 | text | 2026-06-13 |
| `openrouter/free` | The simplest way to get free inference | 200K | free | free | text, image | 2026-01-31 |
| `openrouter/pareto-code` | The Pareto Router maintains a tiered shortlist of strong coding models, ranked by Artificial Analysis coding percentiles | 2M | $-1000000.000 | $-1000000.000 | text | 2026-04-21 |

<details><summary>Older openrouter versions and lines (1)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `openrouter/bodybuilder` | Transform your natural language requests into structured OpenRouter API request objects | 128K | $-1000000.000 | $-1000000.000 | text | 2025-12-04 |

</details>

## particle-ai (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `particle-ai/particle-1` | A token-efficient agentic coding model from Particle AI, post-trained on DeepSeek V4.1 Flash | 1.0M | $0.050 | $0.50 | text, image | 2026-10-28 |

## perceptron (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `perceptron/perceptron-mk1.5` | Perceptron's embodied reasoning model for physical agents | 37K | $0.15 | $1.50 | text, image, video, audio | 2026-09-25 |
| `perceptron/perceptron-mk1` | Perceptron Mk1 (Mark One) is Perceptron's highest-quality vision-language model for video and embodied reasoning.** It accepts image and video inputs… | 33K | $0.15 | $1.50 | text, image, video | 2026-05-12 |

## prism-ml (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `prism-ml/ternary-bonsai-2-27b` | Bonsai 2 27B is a 27B-parameter reasoning model from PrismML derived from Qwen3.8-27B | 262K | $0.075 | $0.50 | text, image | 2026-09-18 |
| `prism-ml/ternary-bonsai-2-9b` | PrismML’s compact, open-weight language model for conversational AI, reasoning, and tool use | 262K | $0.080 | $0.13 | text, image | 2026-10-16 |

## recursive (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `recursive/flint-v1.0:free` | Flint is an open-weight mixture-of-experts model from Recursive, with about 2.5B active parameters out of 40B total, built by Recursive's… | 262K | free | free | text | 2026-10-21 |

## reflection (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `reflection/beam-1.0-501b-a23b` | Beam 1.0 is Reflection's open-weight reasoning model for coding and long-horizon agentic workflows | 1.0M | $0.30 | $1.00 | text | 2026-10-16 |

## rekaai (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `rekaai/reka-edge` | An extremely efficient 7B multimodal vision-language model that accepts image/video+text inputs and generates text outputs | 16K | $0.100 | $0.100 | image, text, video | 2026-03-20 |

## relace (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `relace/relace-search` | The relace-search model uses 4-12 `view_file` and `grep` tools in parallel to explore a codebase and return relevant files to the user request | 256K | $1.00 | $3.00 | text | 2025-12-08 |

## ricursiveai (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `ricursiveai/ripple-65b-preview` | Ripple-65B-Preview by ricursiveAI, a 65B model with native tool calling, served in BF16 | 1.0M | $0.20 | $0.60 | text | 2026-11-04 |

## sakana (4)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `sakana/fugu-ultra-v2` | The higher-performance model in Sakana AI's Fugu family | 1M | $5.00 | $30.00 | text, image, file | 2026-09-11 |
| `sakana/fugu-ultra` | The higher-performance model in Sakana AI's Fugu family | 1M | $5.00 | $30.00 | text, image | 2026-06-24 |
| `sakana/fugu-max` | The cost-performance model in Sakana AI's Fugu family | 1M | $2.00 | $6.00 | text, image, file | 2026-09-11 |
| `sakana/sakana-namazu` | A Japanese-specialized reasoning model from Sakana AI, based on Kimi K2.6 with additional training for Japanese language and business contexts | 262K | $0.95 | $4.00 | text, image, file | 2026-08-10 |

## scaledown (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `scaledown/summarize` | An abstractive summarization model from ScaleDown | 1M | $0.040 | free | text, image, file | 2026-10-13 |
| `scaledown/extract` | A description-driven entity extraction model from ScaleDown | 1M | $0.040 | free | text, image, file | 2026-10-13 |

## stealth (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `stealth/crouton-alpha` | A lightweight model built for scalable coding, agentic, and multimodal workflows | 1.0M | free | free | text, image, video, file, audio | 2026-11-02 |

## stepfun (3)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `stepfun/step-3.7-flash` | StepFun's latest high-efficiency multimodal Mixture-of-Experts model | 262K | $0.20 | $1.15 | text, image, video | 2026-05-28 |
| `stepfun/step-5-preview` | StepFun's flagship model for agentic work, built on a sparse Mixture-of-Experts architecture (27B active / 600B total parameters) | 1.0M | $1.00 | $2.70 | text, image, video | 2026-10-08 |
| `stepfun/step-3.5-flash` | StepFun's most capable open-source foundation model | 262K | $0.100 | $0.30 | text | 2026-01-29 |

## tencent (8)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `tencent/hy4` | The official release of Hy4 delivers comprehensive and substantial improvements over Hy4 Preview, with particularly strong gains in agentic… | 1.0M | $0.42 | $1.25 | text, image | 2026-11-03 |
| `tencent/hy4-preview` | Tencent: Hy4 preview is a mixture-of-experts model from Tencent, with 49B active parameters out of 770B total | 1.0M | $0.83 | $2.50 | text | 2026-08-28 |
| `tencent/hy-mt2-30b-a3b` | Tencent's flagship translation model in the Hy-MT2 family | 8K | $0.074 | $0.29 | text | 2026-08-20 |
| `tencent/hy-mt2-1.8b` | A compact 1.8B-parameter translation model from Tencent | 8K | $0.044 | $0.18 | text | 2026-08-20 |

<details><summary>Older tencent versions and lines (4)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `tencent/hy-mt2-7b` | A 7B-parameter translation model from Tencent | 8K | $0.074 | $0.29 | text | 2026-08-19 |
| `tencent/hy3` | A 295B-parameter Mixture-of-Experts model from Tencent (21B active, 192 experts with top-8 routing) built for reasoning, agentic workflows, and… | 262K | $0.13 | $0.53 | text | 2026-07-06 |
| `tencent/hy3-preview` | A high-efficiency Mixture-of-Experts model from Tencent designed for agentic workflows and production use | 262K | $0.18 | $0.60 | text | 2026-04-22 |
| `tencent/hunyuan-a13b-instruct` | Hunyuan-A13B is a 13B active parameter Mixture-of-Experts (MoE) language model developed by Tencent, with a total parameter count of 80B and support… | 131K | $0.14 | $0.57 | text | 2025-07-08 |

</details>

## thinkingmachines (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `thinkingmachines/inkling` | An open-weight multimodal mixture-of-experts model from Thinking Machines Lab, with 41B active parameters out of 975B total | 524K | $1.00 | $4.05 | text, image, audio | 2026-07-17 |
| `thinkingmachines/inkling-small` | An open-weight multimodal mixture-of-experts model from Thinking Machines Lab, with 12B active parameters out of 276B total | 524K | $0.45 | $1.20 | text, image, audio | 2026-07-30 |

## typesafe (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `typesafe/jev-router` | Jev Router picks the best model and reasoning effort for each request, balancing quality, speed, and cost | 1M | $-1000000.000 | $-1000000.000 | audio, file, image, text, video | 2026-09-25 |

## unbiased (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `unbiased/pareto-26.9` | Pareto is a multimodal composite model built for research, coding, and agentic workflows, while delivering frontier-level performance across a broad… | 262K | $2.50 | $7.50 | text, image | 2026-10-13 |
| `unbiased/pareto-26.10` | Pareto is a multimodal composite model built for research, coding, and agentic workflows, while delivering frontier-level performance across a broad… | 1.0M | $0.80 | $3.20 | text, image | 2026-10-15 |

## upstage (2)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `upstage/solar-pro4` | Upstage's cost-efficient large language model, featuring a 524K context window | 524K | $0.15 | $0.60 | text | 2026-08-10 |
| `upstage/solar-mini4` | Upstage's compact, cost-efficient language model, a 35B-parameter mixture-of-experts with 3B active parameters and a 524K context window | 524K | $0.050 | $0.20 | text | 2026-09-23 |

## writer (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `writer/palmyra-x5` | Writer's most advanced model, purpose-built for building and scaling AI agents across the enterprise | 1.0M | $0.60 | $6.00 | text | 2026-01-21 |

## xiaomi (5)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `xiaomi/mimo-v2.6-pro-ultraspeed` | The fast speed edition of Xiaomi's flagship foundation model, MiMo-V2.6-Pro | 1.0M | $4.35 | $8.70 | text, image, video, audio | 2026-09-21 |
| `xiaomi/mimo-v2.5` | A native omnimodal model by Xiaomi | 1M | $0.40 | $2.00 | text, audio, image, video | 2026-04-22 |
| `xiaomi/mimo-v2.6-pro` | The flagship foundation model developed by Xiaomi | 1.1M | $0.43 | $0.87 | text, image, video, audio | 2026-09-21 |
| `xiaomi/mimo-v2.6-flash` | An open-source foundation model developed by Xiaomi | 1.1M | $0.14 | $0.28 | text, image, video, audio | 2026-09-21 |

<details><summary>Older xiaomi versions and lines (1)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `xiaomi/mimo-v2.5-pro` | Xiaomi’s flagship model, delivering strong performance in general agentic capabilities, complex software engineering, and long-horizon tasks, with… | 1.1M | $0.48 | $1.80 | text | 2026-04-22 |

</details>

## xingchen-agi (1)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `xingchen-agi/xing4.0-29b-a4b:free` | Xing4.0-29B-A4B is an open-weight mixture-of-experts language model from China Telecom's AI team (TeleAI) and the successor to the TeleChat series | 262K | free | free | text | 2026-10-16 |

# Video, audio and multimodal output

## openai (5)

OpenAI's GPT models: general chat, coding and reasoning; "pro" variants think longer for harder problems.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `openai/gpt-5.4-image-2` | GPT-5.4 Image 2 combines OpenAI's GPT-5.4 model with state-of-the-art image generation capabilities from GPT Image 2 | 272K | $8.00 | $15.00 | image, text, file | 2026-04-21 |
| `openai/gpt-audio` | The gpt-audio model is OpenAI's first generally available audio model | 128K | $2.50 | $10.00 | text, audio | 2026-01-19 |
| `openai/gpt-audio-mini` | A cost-efficient version of GPT Audio | 128K | $0.60 | $2.40 | text, audio | 2026-01-19 |

<details><summary>Older openai versions and lines (2)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `openai/gpt-5-image-mini` | GPT-5 Image Mini combines OpenAI's advanced language capabilities, powered by GPT-5 Mini, with GPT Image 1 Mini for efficient image generation | 400K | $2.50 | $2.00 | file, image, text | 2025-10-16 |
| `openai/gpt-5-image` | GPT-5 Image combines OpenAI's GPT-5 model with state-of-the-art image generation capabilities | 400K | $10.00 | $10.00 | image, text, file | 2025-10-14 |

</details>

## google (11)

Google's Gemini: multimodal (text, images, audio, video in), very long context; Pro is the strong tier, Flash the fast one.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `google/gemini-3-pro-image` | Nano Banana Pro is Google’s most advanced image-generation and editing model, built on Gemini 3 Pro | 131K | $2.00 | $12.00 | image, text | 2026-06-17 |
| `google/gemini-omni-1.2-flash` | Google's native multimodal video generation and editing model, the successor to Gemini Omni 1.1 Flash | 131K | $1.50 | $9.00 | text, image, video | 2026-11-05 |
| `google/gemini-nano-banana-2.1` | Nano Banana 2.1 (Gemini Nano Banana 2.1) is Google's image generation and editing model on the Flash tier, succeeding Nano Banana 2 and Nano Banana… | 66K | $1.50 | $7.50 | image, text | 2026-10-06 |
| `google/gemini-3.1-flash-image` | Gemini 3.1 Flash Image, a.k.a | 131K | $0.50 | $3.00 | image, text | 2026-06-17 |
| `google/gemini-3.1-flash-lite-image` | Google's fastest, most cost-efficient Gemini image model, built for high-velocity developer pipelines and rapid-fire visual exploration | 66K | $0.25 | $1.50 | image, text | 2026-06-30 |

<details><summary>Older google versions and lines (6)</summary>

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `google/gemini-omni-1.1-flash` | Google's native multimodal video generation and editing model, the generally available successor to Gemini Omni Flash Preview | 131K | $1.50 | $9.00 | text, image, video | 2026-11-04 |
| `google/lyria-3-pro-preview` | Full-length songs are priced at $0.08 per song | 1.0M | free | free | text, image | 2026-03-30 |
| `google/lyria-3-clip-preview` | 30 second duration clips are priced at $0.04 per clip | 1.0M | free | free | text, image | 2026-03-30 |
| `google/gemini-3.1-flash-image-preview` | Gemini 3.1 Flash Image Preview, a.k.a | 66K | $0.50 | $3.00 | image, text | 2026-02-26 |
| `google/gemini-3-pro-image-preview` | Nano Banana Pro is Google’s most advanced image-generation and editing model, built on Gemini 3 Pro | 66K | $2.00 | $12.00 | image, text | 2025-11-20 |
| `google/gemini-2.5-flash-image` | Gemini 2.5 Flash Image, a.k.a | 33K | $0.30 | $2.50 | image, text | 2025-10-07 |

</details>

## openrouter (3)

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| ⭐ `openrouter/auto` | Auto is OpenRouter’s flagship model router, built for general-purpose usage | 2M | $-1000000.000 | $-1000000.000 | text, image, audio, file, video | 2026-11-05 |
| `openrouter/auto-code` | OpenRouter’s router built specifically for coding agents and harnesses | 2M | $-1000000.000 | $-1000000.000 | text, image, audio, file, video | 2026-11-05 |
| `openrouter/auto-beta` | The experimental version of our Auto Router where we test new improvements | 2M | $-1000000.000 | $-1000000.000 | text, image, audio, file, video | 2026-07-17 |

# Image generation

## openai (10)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `openai/gpt-image-2.6-sunburst` | OpenAI's image generation and editing model optimized for detail: dense compositions, text-heavy layouts, and edits that keep details consistent from… | text, image | image | 2026-11-04 |
| `openai/gpt-image-2.6-flare` | OpenAI's image generation and editing model optimized for speed: fast iteration and interactive text-to-image and image-to-image workflows,… | text, image | image | 2026-11-04 |
| `openai/gpt-image-2.5-sunburst` | An image generation and editing model from OpenAI, positioned as the precision-oriented tier of the GPT Image 2.5 series | text, image | image | 2026-09-08 |

<details><summary>Older openai image models (7)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `openai/gpt-image-2.5-flare` | An image generation and editing model from OpenAI, positioned as the speed-oriented tier of the GPT Image 2.5 series | text, image | image | 2026-09-08 |
| `openai/gpt-image-2` | OpenAI's latest image generation model | text, image | image | 2026-06-23 |
| `openai/gpt-image-1-mini` | A cost-efficient variant of GPT Image 1 for high-quality image generation at reduced latency and cost via OpenAI's dedicated Images API | text, image | image | 2026-06-23 |
| `openai/gpt-image-1` | OpenAI's GPT Image 1 generates and edits images via the dedicated Images API | text, image | image | 2026-06-23 |
| `openai/gpt-5.4-image-2` | GPT-5.4 Image 2 combines OpenAI's GPT-5.4 model with state-of-the-art image generation capabilities from GPT Image 2 | image, text, file | image, text | 2026-04-21 |
| `openai/gpt-5-image-mini` | GPT-5 Image Mini combines OpenAI's advanced language capabilities, powered by GPT-5 Mini, with GPT Image 1 Mini for efficient image generation | file, image, text | image, text | 2025-10-16 |
| `openai/gpt-5-image` | GPT-5 Image combines OpenAI's GPT-5 model with state-of-the-art image generation capabilities | image, text, file | image, text | 2025-10-14 |

</details>

## google (7)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `google/gemini-nano-banana-2.1` | Nano Banana 2.1 (Gemini Nano Banana 2.1) is Google's image generation and editing model on the Flash tier, succeeding Nano Banana 2 and Nano Banana… | image, text | image, text | 2026-10-06 |
| `google/gemini-3.1-flash-lite-image` | Google's fastest, most cost-efficient Gemini image model, built for high-velocity developer pipelines and rapid-fire visual exploration | image, text | image, text | 2026-06-30 |
| `google/gemini-3.1-flash-image` | Gemini 3.1 Flash Image, a.k.a | image, text | image, text | 2026-06-17 |

<details><summary>Older google image models (4)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `google/gemini-3-pro-image` | Nano Banana Pro is Google’s most advanced image-generation and editing model, built on Gemini 3 Pro | image, text | image, text | 2026-06-17 |
| `google/gemini-3.1-flash-image-preview` | Gemini 3.1 Flash Image Preview, a.k.a | image, text | image, text | 2026-02-26 |
| `google/gemini-3-pro-image-preview` | Nano Banana Pro is Google’s most advanced image-generation and editing model, built on Gemini 3 Pro | image, text | image, text | 2025-11-20 |
| `google/gemini-2.5-flash-image` | Gemini 2.5 Flash Image, a.k.a | image, text | image, text | 2025-10-07 |

</details>

## x-ai (1)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `x-ai/grok-imagine-image-2.0` | An image generation and editing model from xAI | text, image | image | 2026-08-11 |

## meta (1)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `meta/muse-image` | An agentic image generation model from Meta that generates and edits images from text and reference images | text, image | image | 2026-08-26 |

## qwen (4)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `qwen/qwen-image-2.1-turbo` | The fast, lower-cost tier of the Qwen Image 2.1 series from Qwen, for image generation and editing where throughput and price matter more than peak | text, image | image | 2026-10-16 |
| `qwen/qwen-image-2.1-pro` | An image generation and editing model from Qwen, the higher-quality tier of the Qwen Image 2.1 series | text, image | image | 2026-10-16 |
| `qwen/qwen-image-3-pro` | An image generation and editing model from Qwen | text, image | image | 2026-08-04 |

<details><summary>Older qwen image models (1)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `qwen/qwen-image-3` | A unified image generation and editing model from Qwen | text, image | image | 2026-08-04 |

</details>

## microsoft (4)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `microsoft/mai-image-2.6` | An image generation and editing model from Microsoft AI, the precision tier of the MAI-Image-2.6 family alongside the faster MAI-Image-2.6 Flash | text, image | image | 2026-09-04 |
| `microsoft/mai-image-2.6-flash` | The lower-latency, lower-cost member of the MAI-Image-2.6 family from Microsoft AI, built for latency-sensitive, high-throughput production image… | text, image | image | 2026-09-04 |
| `microsoft/mai-image-2.5-pro` | Microsoft AI's MAI-Image-2.5 is a high-quality image generation model available via Azure AI Foundry | text, image | image | 2026-07-23 |

<details><summary>Older microsoft image models (1)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `microsoft/mai-image-2.5` | Microsoft AI's MAI-Image-2.5 is a high-quality image generation model available via Azure AI Foundry | text, image | image | 2026-06-02 |

</details>

## black-forest-labs (6)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `black-forest-labs/flux-3-image-klein` | An open-weights image generation and editing model from Black Forest Labs | text, image | image | 2026-10-15 |
| `black-forest-labs/flux-3-image` | FLUX.3 Image is Black Forest Labs' flagship image generation and editing model | text, image | image | 2026-10-01 |
| `black-forest-labs/flux.2-klein-4b` | FLUX.2 [klein] 4B is the fastest and most cost-effective model in the FLUX.2 family, optimized for high-throughput use cases while maintaining… | text, image | image | 2026-01-14 |

<details><summary>Older black-forest-labs image models (3)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `black-forest-labs/flux.2-max` | FLUX.2 [max] is the new top-tier image model from Black Forest Labs, pushing image quality, prompt understanding, and editing consistency to the… | text, image | image | 2025-12-15 |
| `black-forest-labs/flux.2-flex` | FLUX.2 [flex] excels at rendering complex text, typography, and fine details, and supports multi-reference editing in the same unified architecture | text, image | image | 2025-11-24 |
| `black-forest-labs/flux.2-pro` | A high-end image generation and editing model focused on frontier-level visual quality and reliability | text, image | image | 2025-11-24 |

</details>

## bytedance-seed (4)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `bytedance-seed/seedream-5-0-flash` | An image generation and editing model from ByteDance Seed | text, image | image | 2026-10-01 |
| `bytedance-seed/seedream-5-0-lite` | An image generation model from ByteDance Seed | text, image | image | 2026-08-13 |
| `bytedance-seed/seedream-5-0-pro` | An image generation and editing model from ByteDance Seed | text, image | image | 2026-08-12 |

<details><summary>Older bytedance-seed image models (1)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `bytedance-seed/seedream-4.5` | The latest in-house image generation model developed by ByteDance | image, text | image | 2025-12-23 |

</details>

## krea (4)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `krea/k3-edit` | Krea K3 Edit is an image-to-image editing model from Krea | text, image | image | 2026-10-15 |
| `krea/krea-2-large` | Krea's high-capability image generation model, more than twice the size of Krea 2 Medium | text, image | image | 2026-07-20 |
| `krea/krea-2-medium` | Krea's balanced, cost-efficient image generation model and a practical starting point for a broad range of use cases | text, image | image | 2026-07-20 |

<details><summary>Older krea image models (1)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `krea/krea-2-medium-turbo` | A distilled, speed-focused variant of Krea 2 Medium from Krea | text, image | image | 2026-07-20 |

</details>

## recraft (16)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `recraft/recraft-v4.1-flash` | A text-to-image model from Recraft, the speed and cost tier of the V4.1 family | text | image | 2026-09-23 |
| `recraft/recraft-v4-styles-pro` | A style-consistent image generation model from Recraft | text, image | image | 2026-08-26 |
| `recraft/recraft-v4-styles-vector` | A style-consistent image generation model from Recraft | text, image | image | 2026-08-26 |

<details><summary>Older recraft image models (13)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `recraft/recraft-v4-styles-pro-vector` | A style-consistent image generation model from Recraft | text, image | image | 2026-08-26 |
| `recraft/recraft-v4-styles` | A style-consistent image generation model from Recraft | text, image | image | 2026-08-26 |
| `recraft/recraft-v4.1-pro-vector` | The vector (SVG) variant of Recraft V4.1 Pro, tuned for high aesthetics | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-vector` | The vector (SVG) variant of Recraft V4.1, tuned for high aesthetics | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-utility-pro` | A general-purpose image generation model from Recraft | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-utility` | A general-purpose image generation model from Recraft | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-pro` | An image generation model from Recraft tuned for high aesthetics | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1` | An image generation model from Recraft tuned for high aesthetics | text, image | image | 2026-05-13 |
| `recraft/recraft-v4-pro-vector` | The vector (SVG) variant of Recraft V4 Pro | text, image | image | 2026-05-13 |
| `recraft/recraft-v4-vector` | The vector (SVG) variant of Recraft V4 | text, image | image | 2026-05-13 |
| `recraft/recraft-v4-pro` | An image generation model from Recraft | text, image | image | 2026-05-07 |
| `recraft/recraft-v4` | An image generation model from Recraft | text, image | image | 2026-05-07 |
| `recraft/recraft-v3` | An image generation model from Recraft | text, image | image | 2026-05-07 |

</details>

## sourceful (5)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `sourceful/riverflow-v2.6-pro` | Riverflow 2.6 Pro is the flagship model of Sourceful's Riverflow 2.6 lineup, a unified text-to-image and image-to-image family that treats generation… | text, image | image | 2026-10-29 |
| `sourceful/riverflow-v2.5-pro` | The most powerful variant of Sourceful's Riverflow 2.5 lineup, best for top-tier control and quality-sensitive outputs | text, image | image | 2026-06-04 |
| `sourceful/riverflow-v2.5-fast` | The speed-optimized variant of Sourceful's Riverflow 2.5 lineup, best for production deployments and latency-critical workflows | text, image | image | 2026-06-04 |

<details><summary>Older sourceful image models (2)</summary>

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| `sourceful/riverflow-v2-pro` | The most powerful variant of Sourceful's Riverflow 2.0 lineup, best for top-tier control and perfect text rendering | text, image | image | 2026-02-02 |
| `sourceful/riverflow-v2-fast` | The fastest variant of Sourceful's Riverflow 2.0 lineup, best for production deployments and latency-critical workflows | text, image | image | 2026-02-02 |

</details>

## tencent (1)

| Model | What it is for | Input | Output | Added |
|---|---|---|---|---|
| ⭐ `tencent/hy-image-v3.5-preview` | A unified image generation and editing model from Tencent | text, image | image | 2026-10-05 |

Image prices are per endpoint: `ab ai model <id>`.

# Older than a year

Released more than a year ago and not shown above; kept for reference, usually replaced by a newer line of the same vendor.

| Model | What it is for | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|---|
| `amazon/nova-premier-v1` | Amazon Nova Premier is the most capable of Amazon’s multimodal models for complex reasoning tasks and for use as the best teacher for distilling… | 1M | $2.50 | $12.50 | text, image | 2025-10-31 |
| `anthracite-org/magnum-v4-72b` | This is a series of models designed to replicate the prose quality of the Claude 3 models, specifically… | 33K | $2.50 | $5.00 | text | 2024-10-21 |
| `bytedance/ui-tars-1.5-7b` | UI-TARS-1.5 is a multimodal vision-language agent optimized for GUI-based environments, including desktop interfaces, web browsers, mobile systems,… | 128K | $0.100 | $0.20 | image, text | 2025-07-22 |
| `cognitivecomputations/dolphin-mistral-24b-venice-edition` | Venice Uncensored Dolphin Mistral 24B Venice Edition is a fine-tuned variant of Mistral-Small-24B-Instruct-2501, developed by dphn.ai in… | 128K | $0.20 | $0.90 | text | 2025-07-09 |
| `gryphe/mythomax-l2-13b` | One of the highest performing and most popular fine-tunes of Llama 2 13B, with rich descriptions and roleplay | 8K | $0.080 | $0.11 | text | 2023-07-01 |
| `mancer/weaver` | An attempt to recreate Claude-style verbosity, but don't expect the same level of coherence or memory | 8K | $0.40 | $0.75 | text | 2023-08-01 |
| `meta-llama/llama-guard-4-12b` | Llama Guard 4 is a Llama 4 Scout-derived multimodal pretrained model, fine-tuned for content safety classification | 164K | $0.18 | $0.18 | image, text | 2025-04-29 |
| `meta-llama/llama-4-maverick` | Llama 4 Maverick 17B Instruct (128E) is a high-capacity multimodal language model from Meta, built on a mixture-of-experts (MoE) architecture with… | 524K | $0.35 | $1.00 | text, image | 2025-04-05 |
| `meta-llama/llama-4-scout` | Llama 4 Scout 17B Instruct (16E) is a mixture-of-experts (MoE) language model developed by Meta, activating 17 billion parameters out of a total of… | 1.3M | $0.100 | $0.30 | text, image | 2025-04-05 |
| `meta-llama/llama-3.3-70b-instruct` | The Meta Llama 3.3 multilingual large language model (LLM) is a pretrained and instruction tuned generative model in 70B (text in/text out) | 131K | $0.59 | $0.79 | text | 2024-12-06 |
| `microsoft/phi-4` | Microsoft Research Phi-4 is designed to perform well in complex reasoning tasks and can operate efficiently in situations with limited memory or… | 16K | $0.070 | $0.14 | text | 2025-01-10 |
| `morph/morph-v3-large` | Morph's high-accuracy apply model for complex code edits | 262K | $0.90 | $1.90 | text | 2025-07-07 |
| `morph/morph-v3-fast` | Morph's fastest apply model for code edits | 262K | $0.80 | $1.20 | text | 2025-07-07 |
| `nousresearch/hermes-4-405b` | Hermes 4 is a large-scale reasoning model built on Meta-Llama-3.1-405B and released by Nous Research | 131K | $1.00 | $3.00 | text | 2025-08-26 |
| `perplexity/sonar-pro-search` | Exclusively available on the OpenRouter API, Sonar Pro's new Pro Search mode is Perplexity's most advanced agentic search system | 200K | $3.00 | $15.00 | text, image | 2025-10-30 |
| `rekaai/reka-flash-3` | A general-purpose, instruction-tuned large language model with 21 billion parameters, developed by Reka | 66K | $0.100 | $0.20 | text | 2025-03-12 |
| `relace/relace-apply-3` | A specialized code-patching LLM that merges AI-suggested edits straight into your source files | 256K | $0.85 | $1.25 | text | 2025-09-26 |
| `sao10k/l3.1-euryale-70b` | Euryale L3.1 70B v2.2 is a model focused on creative roleplay from Sao10k | 131K | $0.85 | $0.85 | text | 2024-08-27 |
| `sao10k/l3-lunaris-8b` | Lunaris 8B is a versatile generalist and roleplaying model based on Llama 3 | 8K | $0.040 | $0.050 | text | 2024-08-12 |
| `thedrummer/cydonia-24b-v4.1` | Uncensored and creative writing model based on Mistral Small 3.2 24B with good recall, prompt adherence, and intelligence | 131K | $0.30 | $0.50 | text | 2025-09-26 |
| `thedrummer/skyfall-36b-v2` | Skyfall 36B v2 is an enhanced iteration of Mistral Small 2501, specifically fine-tuned for improved creativity, nuanced writing, role-playing, and… | 33K | $0.55 | $0.80 | text | 2025-03-10 |
| `thedrummer/unslopnemo-12b` | UnslopNemo v4.1 is the latest addition from the creator of Rocinante, designed for adventure writing and role-play scenarios | 1.0M | $0.40 | $0.40 | text | 2024-11-08 |
| `undi95/remm-slerp-l2-13b` | A recreation trial of the original MythoMax-L2-B13 but with updated models | 6K | $0.35 | $0.65 | text | 2023-07-21 |
