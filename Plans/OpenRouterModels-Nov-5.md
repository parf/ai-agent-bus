# OpenRouter models, Nov 5 2026

📌 **TL;DR:** every model OpenRouter offers through `#ai`, by kind and vendor. ⭐ marks each vendor's strongest current model (its newest top-priced release); the newest version of every other model line follows, dearest first, and older versions and model lines with no release in the vendor's last six months are folded. `#ai`'s default is marked (default). Details for one: `ab ai model <id>`.

Source: OpenRouter `/api/v1/models` and `/api/v1/images/models`, fetched 2026-11-05 (454 text and media entries, 63 image models). Prices are US$ per million tokens; variants (`:free`, `:batch`, `:thinking`) and `~` aliases are noted on their base model.

## Always-latest aliases

Each routes to the vendor's newest model of that line at request time, e.g. `ab ai ask` with `"model":"~anthropic/claude-sonnet-latest"`.

`~anthropic/claude-fable-latest` · `~anthropic/claude-haiku-latest` · `~anthropic/claude-opus-latest` · `~anthropic/claude-sonnet-latest` · `~deepseek/deepseek-flash-latest` · `~deepseek/deepseek-pro-latest` · `~deepseek/deepseek-v4-flash-latest` · `~google/gemini-flash-latest` · `~google/gemini-pro-latest` · `~moonshotai/kimi-latest` · `~openai/gpt-astra-latest` · `~openai/gpt-luna-latest` · `~openai/gpt-mini-latest` · `~openai/gpt-sol-latest` · `~openai/gpt-terra-latest` · `~unbiased/pareto-latest` · `~x-ai/grok-latest` · `~z-ai/glm-flash-latest` · `~z-ai/glm-latest`

## Text and chat

### anthropic (15)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `anthropic/claude-fable-5.1` · :batch | 1M | $10.00 | $50.00 | text, image, file | 2026-09-01 |
| `anthropic/claude-opus-5.5` · :batch | 1M | $4.00 | $20.00 | text, image, file | 2026-09-22 |
| `anthropic/claude-sonnet-5.5` · :batch | 1M | $2.00 | $10.00 | text, image, file | 2026-09-28 |
| **default** `anthropic/claude-haiku-5.5` · :batch | 1M | $0.100 | $0.50 | text, image, file | 2026-10-07 |

<details><summary>Older anthropic versions and lines (11)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `anthropic/claude-opus-5` · :batch | 1M | $5.00 | $25.00 | text, image, file | 2026-07-24 |
| `anthropic/claude-sonnet-5` · :batch | 1M | $2.00 | $10.00 | text, image, file | 2026-06-30 |
| `anthropic/claude-fable-5` · :batch | 1M | $10.00 | $50.00 | text, image, file | 2026-06-09 |
| `anthropic/claude-opus-4.8` · :batch | 1M | $5.00 | $25.00 | text, image, file | 2026-05-27 |
| `anthropic/claude-opus-4.7` · :batch | 1M | $5.00 | $25.00 | text, image, file | 2026-04-16 |
| `anthropic/claude-sonnet-4.6` · :batch | 1M | $3.00 | $15.00 | text, image, file | 2026-02-17 |
| `anthropic/claude-opus-4.6` · :batch | 1M | $5.00 | $25.00 | text, image, file | 2026-02-04 |
| `anthropic/claude-opus-4.5` · :batch | 200K | $5.00 | $25.00 | file, image, text | 2025-11-24 |
| `anthropic/claude-haiku-4.5` · :batch | 200K | $1.00 | $5.00 | text, image, file | 2025-10-15 |
| `anthropic/claude-sonnet-4.5` · :batch | 1M | $3.00 | $15.00 | text, image, file | 2025-09-29 |
| `anthropic/claude-opus-4.1` · :batch | 200K | $15.00 | $75.00 | image, text, file | 2025-08-05 |

</details>

### openai (56)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `openai/gpt-6-astra` · :batch | 1.1M | $10.00 | $50.00 | file, image, text | 2026-09-04 |
| `openai/gpt-6-astra-pro` · :batch | 1.1M | $10.00 | $50.00 | file, image, text | 2026-09-04 |
| `openai/gpt-chat-latest` | 400K | $5.00 | $30.00 | text, image, file | 2026-05-05 |
| `openai/gpt-6.1-sol-pro` · :batch | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-29 |
| `openai/gpt-6.1-sol` · :batch | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-29 |
| `openai/gpt-6.1-terra-pro` · :batch | 1.1M | $1.00 | $5.00 | file, image, text | 2026-10-26 |
| `openai/gpt-6.1-terra` · :batch | 1.1M | $1.00 | $5.00 | file, image, text | 2026-10-26 |
| `openai/gpt-6-luna-pro` · :batch | 1.1M | $0.100 | $0.50 | file, image, text | 2026-09-22 |
| `openai/gpt-6-luna` · :batch | 1.1M | $0.100 | $0.50 | file, image, text | 2026-09-22 |

<details><summary>Older openai versions and lines (47)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `openai/gpt-6-sol-pro` · :batch | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-22 |
| `openai/gpt-6-sol` · :batch | 1.1M | $2.00 | $10.00 | file, image, text | 2026-09-22 |
| `openai/gpt-5.6-luna-pro` · :batch | 1.1M | $0.20 | $1.20 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-luna` · :batch | 1.1M | $0.20 | $1.20 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-terra-pro` · :batch | 1.1M | $2.00 | $12.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-terra` · :batch | 1.1M | $2.00 | $12.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-sol-pro` · :batch | 1.1M | $4.00 | $20.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.6-sol` · :batch | 1.1M | $4.00 | $20.00 | file, image, text | 2026-07-09 |
| `openai/gpt-5.5-pro` · :batch | 1.1M | $30.00 | $180.00 | file, image, text | 2026-04-24 |
| `openai/gpt-5.5` · :batch | 1.1M | $5.00 | $30.00 | file, image, text | 2026-04-24 |
| `openai/gpt-5.4-nano` · :batch | 400K | $0.20 | $1.25 | file, image, text | 2026-03-17 |
| `openai/gpt-5.4-mini` · :batch | 400K | $0.75 | $4.50 | file, image, text | 2026-03-17 |
| `openai/gpt-5.4-pro` · :batch | 1.1M | $30.00 | $180.00 | text, image, file | 2026-03-05 |
| `openai/gpt-5.4` · :batch | 1.1M | $2.50 | $15.00 | text, image, file | 2026-03-05 |
| `openai/gpt-5.3-codex` | 400K | $1.75 | $14.00 | text, image, file | 2026-02-24 |
| `openai/gpt-5.2-codex` | 400K | $1.75 | $14.00 | text, image | 2026-01-14 |
| `openai/gpt-5.2-chat` | 128K | $1.75 | $14.00 | file, image, text | 2025-12-10 |
| `openai/gpt-5.2-pro` · :batch | 400K | $21.00 | $168.00 | image, text, file | 2025-12-10 |
| `openai/gpt-5.2` · :batch | 400K | $1.75 | $14.00 | file, image, text | 2025-12-10 |
| `openai/gpt-5.1-codex-max` | 400K | $1.25 | $10.00 | text, image | 2025-12-04 |
| `openai/gpt-5.1` · :batch | 400K | $1.25 | $10.00 | image, text, file | 2025-11-13 |
| `openai/gpt-5.1-codex` | 400K | $1.25 | $10.00 | text, image | 2025-11-13 |
| `openai/gpt-5.1-codex-mini` | 400K | $0.25 | $2.00 | image, text | 2025-11-13 |
| `openai/gpt-oss-safeguard-20b` | 131K | $0.075 | $0.30 | text | 2025-10-29 |
| `openai/gpt-5-pro` · :batch | 400K | $15.00 | $120.00 | image, text, file | 2025-10-06 |
| `openai/gpt-5` · :batch | 400K | $1.25 | $10.00 | text, image, file | 2025-08-07 |
| `openai/gpt-5-mini` · :batch | 400K | $0.25 | $2.00 | text, image, file | 2025-08-07 |
| `openai/gpt-5-nano` · :batch | 400K | $0.050 | $0.40 | text, image, file | 2025-08-07 |
| `openai/gpt-oss-120b` · :batch | 131K | $0.016 | $0.059 | text | 2025-08-05 |
| `openai/gpt-oss-20b` · :batch | 131K | $0.030 | $0.13 | text | 2025-08-05 |
| `openai/o3-pro` | 200K | $20.00 | $80.00 | text, file, image | 2025-06-10 |
| `openai/o3` · :batch | 200K | $2.00 | $8.00 | image, text, file | 2025-04-16 |
| `openai/o4-mini:batch` | 200K | $0.55 | $2.20 | image, text, file | 2025-04-16 |
| `openai/gpt-4.1` · :batch | 1.0M | $2.00 | $8.00 | image, text, file | 2025-04-14 |
| `openai/gpt-4.1-mini` · :batch | 1.0M | $0.40 | $1.60 | image, text, file | 2025-04-14 |
| `openai/gpt-4.1-nano` · :batch | 1.0M | $0.100 | $0.40 | image, text, file | 2025-04-14 |
| `openai/o3-mini:batch` | 200K | $0.55 | $2.20 | text, file | 2025-01-31 |
| `openai/gpt-4o-2024-11-20` | 128K | $2.50 | $10.00 | text, image, file | 2024-11-20 |
| `openai/gpt-4o-2024-08-06` | 128K | $2.50 | $10.00 | text, image, file | 2024-08-05 |
| `openai/gpt-4o-mini` · :batch | 128K | $0.15 | $0.60 | text, image, file | 2024-07-17 |
| `openai/gpt-4o-mini-2024-07-18` | 128K | $0.15 | $0.60 | text, image, file | 2024-07-17 |
| `openai/gpt-4o` · :batch | 128K | $2.50 | $10.00 | text, image, file | 2024-05-12 |
| `openai/gpt-4o-2024-05-13` | 128K | $5.00 | $15.00 | text, image, file | 2024-05-12 |
| `openai/gpt-4-turbo:batch` | 128K | $5.00 | $15.00 | text, image | 2024-04-08 |
| `openai/gpt-3.5-turbo-0613` | 4K | $1.00 | $2.00 | text | 2024-01-24 |
| `openai/gpt-3.5-turbo-16k` | 16K | $3.00 | $4.00 | text | 2023-08-27 |
| `openai/gpt-4` | 8K | $30.00 | $60.00 | text | 2023-05-27 |

</details>

### google (21)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `google/gemini-4-argon` · :batch | 1.0M | $2.00 | $10.00 | audio, file, image, text, video | 2026-10-13 |
| `google/gemini-3.8-flash` · :batch | 1.0M | $0.83 | $4.12 | text, image, video, file, audio | 2026-09-02 |
| `google/gemini-3.5-flash-lite` · :batch | 1.0M | $0.30 | $2.50 | text, image, video, file, audio | 2026-07-21 |

<details><summary>Older google versions and lines (18)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `google/gemini-3.7-flash` · :batch | 1.0M | $0.75 | $3.75 | text, image, video, file, audio | 2026-08-13 |
| `google/gemini-3.6-flash` · :batch | 1.0M | $0.75 | $3.75 | text, image, video, file, audio | 2026-07-21 |
| `google/gemini-3.5-flash` · :batch | 1.0M | $1.50 | $9.00 | text, image, video, file, audio | 2026-05-19 |
| `google/gemini-3.1-flash-lite` · :batch | 1.0M | $0.25 | $1.50 | text, image, video, file, audio | 2026-05-07 |
| `google/gemma-4-26b-a4b-it` · :free | 262K | $0.076 | $0.26 | image, text, video | 2026-04-03 |
| `google/gemma-4-31b-it` · :free | 262K | $0.20 | $0.40 | image, text, video | 2026-04-02 |
| `google/gemini-3.1-flash-lite-preview` | 1.0M | $0.25 | $1.50 | text, image, video, file, audio | 2026-03-02 |
| `google/gemini-3.1-pro-preview-customtools` | 1.0M | $2.00 | $12.00 | text, audio, image, video, file | 2026-02-25 |
| `google/gemini-3.1-pro-preview` · :batch | 1.0M | $2.00 | $12.00 | audio, file, image, text, video | 2026-02-19 |
| `google/gemini-3-flash-preview` · :batch | 1.0M | $0.50 | $3.00 | text, image, file, audio, video | 2025-12-17 |
| `google/gemini-2.5-flash-lite` | 1.0M | $0.100 | $0.40 | text, image, file, audio, video | 2025-07-22 |
| `google/gemini-2.5-flash` | 1.0M | $0.30 | $2.50 | file, image, text, audio, video | 2025-06-17 |
| `google/gemini-2.5-pro` | 1.0M | $1.25 | $10.00 | text, image, file, audio, video | 2025-06-17 |
| `google/gemini-2.5-pro-preview` | 1.0M | $1.25 | $10.00 | file, image, text, audio | 2025-06-05 |
| `google/gemma-3-4b-it` | 131K | $0.050 | $0.100 | text, image | 2025-03-13 |
| `google/gemma-3-12b-it` | 131K | $0.050 | $0.15 | text, image | 2025-03-13 |
| `google/gemma-3-27b-it` | 131K | $0.080 | $0.35 | text, image | 2025-03-12 |
| `google/gemma-2-27b-it` | 8K | $0.65 | $0.65 | text | 2024-07-12 |

</details>

### x-ai (7)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `x-ai/grok-4.7` · :batch | 500K | $2.00 | $6.00 | text, image, file | 2026-09-21 |
| `x-ai/grok-4.20-multi-agent` | 2M | $1.25 | $2.50 | text, image, file | 2026-03-31 |
| `x-ai/grok-build-0.1` | 256K | $1.00 | $2.00 | text, image, file | 2026-05-20 |

<details><summary>Older x-ai versions and lines (4)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `x-ai/grok-4.6` · :batch | 500K | $2.00 | $6.00 | text, image, file | 2026-08-12 |
| `x-ai/grok-4.5` | 500K | $2.00 | $6.00 | text, image, file | 2026-07-08 |
| `x-ai/grok-4.3` · :batch | 1M | $1.25 | $2.50 | text, image, file | 2026-04-30 |
| `x-ai/grok-4.20` | 2M | $1.25 | $2.50 | text, image, file | 2026-03-31 |

</details>

### meta-llama (8)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `meta-llama/llama-4-maverick` | 524K | $0.35 | $1.00 | text, image | 2025-04-05 |
| `meta-llama/llama-3.3-70b-instruct` | 131K | $0.59 | $0.79 | text | 2024-12-06 |
| `meta-llama/llama-4-scout` | 1.3M | $0.100 | $0.30 | text, image | 2025-04-05 |
| `meta-llama/llama-guard-4-12b` | 164K | $0.18 | $0.18 | image, text | 2025-04-29 |

<details><summary>Older meta-llama versions and lines (4)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `meta-llama/llama-3.2-1b-instruct` | 60K | $0.027 | $0.20 | text | 2024-09-24 |
| `meta-llama/llama-3.2-3b-instruct` | 131K | $0.051 | $0.34 | text | 2024-09-24 |
| `meta-llama/llama-3.1-70b-instruct` | 131K | $0.72 | $0.72 | text | 2024-07-22 |
| `meta-llama/llama-3.1-8b-instruct` | 131K | $0.050 | $0.080 | text | 2024-07-22 |

</details>

### meta (8)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `meta/muse-wonder` | 1.0M | $4.00 | $20.00 | text, image, video, file, audio | 2026-10-12 |
| `meta/muse-spark-open` | 1.0M | $1.25 | $4.25 | text, image, video, file, audio | 2026-10-15 |
| `meta/muse-spark-1.3` | 1.0M | $1.25 | $4.25 | text, image, video, file | 2026-09-02 |
| `meta/muse-glimmer-30b` | 131K | $0.30 | $1.20 | text, image | 2026-08-09 |
| `meta/muse-spark-1.3-contributor` | 1.0M | $0.100 | $0.20 | text, image, video, file | 2026-09-02 |

<details><summary>Older meta versions and lines (3)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `meta/muse-spark-1.2-contributor` | 1.0M | $0.100 | $0.20 | text, image, video, file | 2026-08-21 |
| `meta/muse-spark-1.2` | 1.0M | $1.25 | $4.25 | text, image, video, file | 2026-08-05 |
| `meta/muse-spark-1.1` | 1.0M | $1.25 | $4.25 | text, image, video, file | 2026-07-16 |

</details>

### mistralai (20)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `mistralai/mistral-medium-3-5` · :batch | 262K | $1.50 | $7.50 | text, image, file | 2026-04-30 |
| `mistralai/mistral-large-4-0` | 1.0M | $0.70 | $2.10 | text, image | 2026-10-06 |

<details><summary>Older mistralai versions and lines (18)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `mistralai/mistral-small-2603` | 262K | $0.15 | $0.60 | text, image | 2026-03-16 |
| `mistralai/devstral-2512` | 262K | $0.40 | $2.00 | text, file | 2025-12-09 |
| `mistralai/ministral-14b-2512` | 262K | $0.20 | $0.20 | text, image | 2025-12-02 |
| `mistralai/ministral-8b-2512` | 262K | $0.15 | $0.15 | text, image | 2025-12-02 |
| `mistralai/ministral-3b-2512` | 131K | $0.100 | $0.100 | text, image | 2025-12-02 |
| `mistralai/mistral-large-2512` | 262K | $0.50 | $1.50 | text, image, file | 2025-12-01 |
| `mistralai/voxtral-small-24b-2507` | 33K | $0.100 | $0.30 | text, audio, file | 2025-10-30 |
| `mistralai/mistral-medium-3.1` | 131K | $0.40 | $2.00 | text, image, file | 2025-08-13 |
| `mistralai/codestral-2508` | 256K | $0.30 | $0.90 | text, file | 2025-08-01 |
| `mistralai/mistral-small-3.2-24b-instruct` | 256K | $0.075 | $0.20 | image, text | 2025-06-20 |
| `mistralai/mistral-medium-3` | 131K | $0.40 | $2.00 | text, image, file | 2025-05-07 |
| `mistralai/mistral-small-3.1-24b-instruct` | 128K | $0.35 | $0.55 | text, image | 2025-03-17 |
| `mistralai/mistral-saba` | 33K | $0.20 | $0.60 | text, file | 2025-02-17 |
| `mistralai/mistral-small-24b-instruct-2501` | 33K | $0.050 | $0.080 | text | 2025-01-30 |
| `mistralai/mistral-large-2407` | 131K | $2.00 | $6.00 | text, file | 2024-11-18 |
| `mistralai/mistral-nemo` | 131K | $0.029 | $0.030 | text | 2024-07-18 |
| `mistralai/mixtral-8x22b-instruct` | 66K | $2.00 | $6.00 | text, file | 2024-04-16 |
| `mistralai/mistral-large` | 128K | $2.00 | $6.00 | text, file | 2024-02-25 |

</details>

### deepseek (14)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `deepseek/deepseek-v4-pro-0813` | 1.0M | $0.10 | $4.11 | text | 2026-08-12 |
| `deepseek/deepseek-v4-flash-vision-exp` | 1.0M | $0.44 | $1.32 | text, image | 2026-08-21 |
| `deepseek/deepseek-v4.1-flash-1017` | 1.0M | $0.30 | $1.20 | text, image | 2026-10-17 |

<details><summary>Older deepseek versions and lines (11)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `deepseek/deepseek-v4.1-flash` · :batch | 1.0M | $0.045 | $0.50 | text, image | 2026-09-10 |
| `deepseek/deepseek-v4-flash-0731` | 1.0M | $0.059 | $0.30 | text | 2026-07-31 |
| `deepseek/deepseek-v4-pro` | 1.0M | $0.96 | $1.91 | text | 2026-04-23 |
| `deepseek/deepseek-v4-flash` | 1.0M | $0.091 | $0.18 | text | 2026-04-23 |
| `deepseek/deepseek-v3.2` | 164K | $0.26 | $0.38 | text | 2025-12-01 |
| `deepseek/deepseek-v3.2-exp` | 164K | $0.27 | $0.41 | text | 2025-09-29 |
| `deepseek/deepseek-v3.1-terminus` | 164K | $0.27 | $1.00 | text | 2025-09-22 |
| `deepseek/deepseek-chat-v3.1` | 164K | $0.25 | $0.95 | text | 2025-08-21 |
| `deepseek/deepseek-r1-0528` | 164K | $0.50 | $2.15 | text | 2025-05-28 |
| `deepseek/deepseek-chat-v3-0324` | 164K | $0.29 | $1.14 | text | 2025-03-24 |
| `deepseek/deepseek-chat` | 164K | $0.26 | $1.03 | text | 2024-12-26 |

</details>

### qwen (41)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `qwen/qwen3.8-max-prime` | 1M | $4.00 | $12.00 | text, image, video | 2026-09-23 |
| `qwen/qwen3.8-max-0902` | 1M | $2.00 | $6.00 | text, image, video | 2026-09-03 |
| `qwen/qwen3.8-2.4t-a95b` | 1.0M | $2.00 | $6.00 | text | 2026-08-12 |
| `qwen/qwen3.8-27b` | 1.0M | $0.15 | $1.88 | text, image, video | 2026-08-14 |
| `qwen/qwen3.7-plus` · :batch | 1M | $0.32 | $1.28 | text, image | 2026-06-03 |
| `qwen/qwen3.6-35b-a3b` | 262K | $0.100 | $0.90 | text, image, video | 2026-04-26 |
| `qwen/qwen3.8-omni-flash` | 1M | $0.15 | $0.47 | text, image, audio, video | 2026-09-20 |
| `qwen/qwen3.8-flash` | 1M | $0.15 | $0.47 | text, image, video | 2026-08-26 |

<details><summary>Older qwen versions and lines (33)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `qwen/qwen3.7-flash` | 1M | $0.030 | $0.13 | text, image, video | 2026-07-27 |
| `qwen/qwen3.7-max` | 1M | $1.48 | $4.42 | text | 2026-05-21 |
| `qwen/qwen3.5-plus-20260420` | 1M | $0.30 | $1.80 | text, image, video | 2026-04-26 |
| `qwen/qwen3.6-flash` | 1M | $0.19 | $1.12 | text, image, video | 2026-04-26 |
| `qwen/qwen3.6-27b` | 262K | $0.45 | $2.70 | text, image, video | 2026-04-26 |
| `qwen/qwen3.6-plus` | 1M | $0.33 | $1.95 | text, image, video | 2026-04-02 |
| `qwen/qwen3.5-9b` | 262K | $0.17 | $0.25 | text, image, video | 2026-03-10 |
| `qwen/qwen3.5-35b-a3b` | 262K | $0.14 | $1.00 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-27b` | 262K | $0.26 | $2.60 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-122b-a10b` | 262K | $0.26 | $2.08 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-flash-02-23` | 1M | $0.065 | $0.26 | text, image, video | 2026-02-25 |
| `qwen/qwen3.5-plus-02-15` | 1M | $0.26 | $1.56 | text, image, video | 2026-02-16 |
| `qwen/qwen3.5-397b-a17b` | 262K | $0.50 | $3.60 | text, image, video | 2026-02-16 |
| `qwen/qwen3-coder-next` | 262K | $0.12 | $0.80 | text | 2026-02-03 |
| `qwen/qwen3-vl-8b-instruct` | 262K | $0.25 | $0.75 | image, text | 2025-10-14 |
| `qwen/qwen3-vl-30b-a3b-thinking` | 262K | $0.29 | $1.00 | text, image | 2025-10-06 |
| `qwen/qwen3-vl-30b-a3b-instruct` | 262K | $0.15 | $0.60 | text, image | 2025-10-06 |
| `qwen/qwen3-vl-235b-a22b-instruct` | 262K | $0.21 | $1.90 | text, image | 2025-09-23 |
| `qwen/qwen3-coder-flash` | 1M | $0.20 | $0.97 | text | 2025-09-17 |
| `qwen/qwen3-next-80b-a3b-instruct` | 262K | $0.090 | $1.10 | text | 2025-09-11 |
| `qwen/qwen3-coder-30b-a3b-instruct` | 262K | $0.15 | $0.60 | text | 2025-07-31 |
| `qwen/qwen3-30b-a3b-instruct-2507` | 262K | $0.100 | $0.30 | text | 2025-07-29 |
| `qwen/qwen3-235b-a22b-thinking-2507` | 128K | $0.45 | $3.50 | text | 2025-07-25 |
| `qwen/qwen3-coder` | 262K | $0.30 | $1.00 | text | 2025-07-22 |
| `qwen/qwen3-235b-a22b-2507` | 262K | $0.14 | $0.80 | text | 2025-07-21 |
| `qwen/qwen3-30b-a3b` | 41K | $0.12 | $0.50 | text | 2025-04-28 |
| `qwen/qwen3-14b` | 41K | $0.12 | $0.24 | text | 2025-04-28 |
| `qwen/qwen3-32b` | 131K | $0.080 | $0.28 | text | 2025-04-28 |
| `qwen/qwen2.5-vl-72b-instruct` | 128K | $0.80 | $1.00 | text, image | 2025-02-01 |
| `qwen/qwen-plus` | 1M | $0.26 | $0.78 | text | 2025-02-01 |
| `qwen/qwen-2.5-coder-32b-instruct` | 33K | $0.66 | $1.00 | text | 2024-11-11 |
| `qwen/qwen-2.5-7b-instruct` | 33K | $0.100 | $0.20 | text | 2024-10-15 |
| `qwen/qwen-2.5-72b-instruct` | 33K | $0.36 | $0.40 | text | 2024-09-18 |

</details>

### moonshotai (8)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `moonshotai/kimi-k3` · :batch | 1.0M | $0.19 | $16.00 | text, image, video | 2026-07-16 |
| `moonshotai/kimi-k3.1-pro` | 1.0M | $2.00 | $10.00 | text, image, video | 2026-10-23 |
| `moonshotai/kimi-k2.7-code` | 262K | $0.60 | $3.50 | text, image | 2026-06-12 |

<details><summary>Older moonshotai versions and lines (5)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `moonshotai/kimi-k2.6` | 262K | $0.65 | $3.41 | text, image | 2026-04-20 |
| `moonshotai/kimi-k2.5` | 262K | $0.50 | $2.50 | text, image | 2026-01-26 |
| `moonshotai/kimi-k2-thinking` | 262K | $0.60 | $2.50 | text | 2025-11-06 |
| `moonshotai/kimi-k2-0905` | 262K | $0.60 | $2.50 | text | 2025-09-04 |
| `moonshotai/kimi-k2` | 131K | $0.57 | $2.30 | text | 2025-07-11 |

</details>

### z-ai (17)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `z-ai/glm-5.3-prime` | 1M | $2.80 | $8.80 | text | 2026-09-23 |
| `z-ai/glm-5.3` · :batch | 1.0M | $0.070 | $1.40 | text | 2026-08-18 |
| `z-ai/glm-5.3-flashx-legal` | 1.0M | $0.37 | $1.25 | text, image, video | 2026-11-02 |
| `z-ai/glm-5.3-flashx` | 1.0M | $0.37 | $1.25 | text, image, video | 2026-09-18 |
| `z-ai/glm-5.3-flash` · :batch | 1.0M | $0.15 | $0.50 | text, image, video | 2026-08-26 |

<details><summary>Older z-ai versions and lines (12)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `z-ai/glm-5.2` | 1.0M | $0.006 | $6.80 | text | 2026-06-16 |
| `z-ai/glm-5.1` | 205K | $1.40 | $4.40 | text | 2026-04-07 |
| `z-ai/glm-5v-turbo` | 203K | $1.20 | $4.00 | image, text, video | 2026-04-01 |
| `z-ai/glm-5-turbo` | 203K | $1.20 | $4.00 | text | 2026-03-15 |
| `z-ai/glm-5` | 205K | $1.00 | $3.20 | text | 2026-02-11 |
| `z-ai/glm-4.7-flash` | 200K | $0.061 | $0.40 | text | 2026-01-19 |
| `z-ai/glm-4.7` | 205K | $0.60 | $2.20 | text | 2025-12-21 |
| `z-ai/glm-4.6v` | 131K | $0.30 | $0.90 | image, text, video | 2025-12-08 |
| `z-ai/glm-4.6` | 205K | $0.50 | $2.00 | text | 2025-09-30 |
| `z-ai/glm-4.5v` | 66K | $0.60 | $1.80 | text, image | 2025-08-11 |
| `z-ai/glm-4.5` | 131K | $0.60 | $2.20 | text | 2025-07-25 |
| `z-ai/glm-4.5-air` | 131K | $0.13 | $0.85 | text | 2025-07-25 |

</details>

### minimax (5)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `minimax/minimax-m3` | 1.0M | $0.30 | $1.20 | text, image, video | 2026-05-31 |
| `minimax/minimax-m3.1-flash` | 1.0M | $0.15 | $0.60 | text, image, video | 2026-10-18 |

<details><summary>Older minimax versions and lines (3)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `minimax/minimax-m2.7` | 205K | $0.21 | $0.84 | text | 2026-03-18 |
| `minimax/minimax-m2.5` | 205K | $0.27 | $0.95 | text | 2026-02-12 |
| `minimax/minimax-m1` | 1M | $0.55 | $2.20 | text | 2025-06-17 |

</details>

### nvidia (8)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `nvidia/nemotron-3-ultra-550b-a55b` · :free | 262K | $0.60 | $2.40 | text | 2026-06-04 |
| `nvidia/nemotron-3.5-super-vl` · :free | 1.0M | $0.15 | $0.50 | text, image, video | 2026-10-15 |
| `nvidia/nemotron-3.5-lightning` · :free | 262K | $0.070 | $0.20 | text | 2026-08-11 |
| `nvidia/nemotron-3.5-content-safety` · :free | 131K | $0.20 | $0.20 | text, image | 2026-06-04 |
| `nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free` | 256K | free | free | text, audio, image, video | 2026-04-28 |
| `nvidia/switchyard` | 1M | $-1000000.000 | $-1000000.000 | text | 2026-09-21 |

<details><summary>Older nvidia versions and lines (2)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `nvidia/nemotron-3-super-120b-a12b` | 1.0M | $0.30 | $2.40 | text | 2026-03-11 |
| `nvidia/nemotron-3-nano-30b-a3b` | 262K | $0.060 | $0.24 | text | 2025-12-14 |

</details>

### amazon (5)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `amazon/nova-premier-v1` | 1M | $2.50 | $12.50 | text, image | 2025-10-31 |
| `amazon/nova-2-lite-v1` | 1M | $0.30 | $2.50 | text, image, video, file | 2025-12-02 |

<details><summary>Older amazon versions and lines (3)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `amazon/nova-lite-v1` | 300K | $0.060 | $0.24 | text, image | 2024-12-05 |
| `amazon/nova-micro-v1` | 128K | $0.035 | $0.14 | text | 2024-12-05 |
| `amazon/nova-pro-v1` | 300K | $0.80 | $3.20 | text, image | 2024-12-05 |

</details>

### microsoft (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `microsoft/phi-4` | 16K | $0.070 | $0.14 | text | 2025-01-10 |

### cohere (6)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `cohere/command-a-plus` | 192K | $0.30 | $1.50 | text, image | 2026-09-22 |
| `cohere/north-mini-code:free` | 256K | free | free | text | 2026-06-17 |

<details><summary>Older cohere versions and lines (4)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `cohere/command-a` | 256K | $2.50 | $10.00 | text | 2025-03-13 |
| `cohere/command-r7b-12-2024` | 128K | $0.037 | $0.15 | text | 2024-12-14 |
| `cohere/command-r-08-2024` | 128K | $0.15 | $0.60 | text | 2024-08-29 |
| `cohere/command-r-plus-08-2024` | 128K | $2.50 | $10.00 | text | 2024-08-29 |

</details>

### aion-labs (6)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `aion-labs/aion-3.5` | 262K | $3.00 | $6.00 | text | 2026-09-23 |
| `aion-labs/aion-3.5-mini` | 262K | $0.70 | $1.40 | text | 2026-09-23 |

<details><summary>Older aion-labs versions and lines (4)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `aion-labs/aion-3.0-mini` | 131K | $0.70 | $1.40 | text | 2026-07-07 |
| `aion-labs/aion-3.0` | 131K | $3.00 | $6.00 | text | 2026-07-07 |
| `aion-labs/aion-2.0` | 131K | $0.80 | $1.60 | text | 2026-02-23 |
| `aion-labs/aion-rp-llama-3.1-8b` | 33K | $0.80 | $1.60 | text | 2025-02-04 |

</details>

### anthracite-org (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `anthracite-org/magnum-v4-72b` | 33K | $2.50 | $5.00 | text | 2024-10-21 |

### bytedance (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `bytedance/ui-tars-1.5-7b` | 128K | $0.100 | $0.20 | image, text | 2025-07-22 |

### bytedance-seed (6)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `bytedance-seed/seed-2.0-code` | 262K | $0.50 | $3.00 | text, image, video | 2026-08-12 |
| `bytedance-seed/seed-2-1-turbo` | 262K | $0.50 | $2.50 | text, image, video | 2026-08-12 |
| `bytedance-seed/seed-2.0-lite` | 262K | $0.25 | $2.00 | text, image, video | 2026-03-10 |
| `bytedance-seed/seed-2.0-mini` | 262K | $0.100 | $0.40 | text, image, video | 2026-02-26 |

<details><summary>Older bytedance-seed versions and lines (2)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `bytedance-seed/seed-1.6-flash` | 262K | $0.075 | $0.30 | image, text, video | 2025-12-23 |
| `bytedance-seed/seed-1.6` | 262K | $0.25 | $2.00 | image, text, video | 2025-12-23 |

</details>

### cognitivecomputations (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `cognitivecomputations/dolphin-mistral-24b-venice-edition` | 128K | $0.20 | $0.90 | text | 2025-07-09 |

### dots-studio (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `dots-studio/dots-3-note-preview:free` | 512K | free | free | text, image | 2026-08-14 |

### elorian (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `elorian/glimpse` | 262K | $0.60 | $3.00 | text, image, video | 2026-10-16 |

### fireworks (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `fireworks/ember-1` | 1.0M | $3.00 | $15.00 | text, image | 2026-09-23 |

### gryphe (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `gryphe/mythomax-l2-13b` | 8K | $0.080 | $0.11 | text | 2023-07-01 |

### ibm-granite (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `ibm-granite/granite-4.2-8b` | 131K | $0.060 | $0.25 | text | 2026-08-31 |

### inception (3)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `inception/mercury-3` | 1M | $0.100 | $0.35 | text | 2026-10-16 |
| `inception/mercury-2.5` | 260K | $0.040 | $0.15 | text | 2026-09-08 |
| `inception/mercury-2` | 128K | $0.25 | $0.75 | text | 2026-03-04 |

### inclusionai (4)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `inclusionai/ling-3.1-flash` | 1M | $0.075 | $0.22 | text | 2026-10-02 |
| `inclusionai/ling-3.0-flash-vl` | 131K | $0.060 | $0.18 | text, image, video | 2026-09-10 |
| `inclusionai/ling-3.0-tiny` | 262K | $0.015 | $0.045 | text | 2026-08-06 |

<details><summary>Older inclusionai versions and lines (1)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `inclusionai/ling-3.0-flash` | 262K | $0.021 | $0.063 | text | 2026-07-23 |

</details>

### inference-net (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `inference-net/schematron-v2-small` | 128K | $0.050 | $0.23 | text | 2026-09-11 |
| `inference-net/schematron-v2-turbo` | 128K | $0.030 | $0.15 | text | 2026-09-11 |

### liquid (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `liquid/lfm-2.5-24b-a2b` | 128K | $0.050 | $0.70 | text | 2026-10-13 |
| `liquid/lfm-2.5-2.6b` · :free | 128K | $0.35 | $0.50 | text | 2026-08-11 |

### mancer (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `mancer/weaver` | 8K | $0.40 | $0.75 | text | 2023-08-01 |

### meituan (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `meituan/longcat-2.0` | 1.0M | $0.30 | $1.20 | text | 2026-07-20 |

### moondream (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `moondream/moondream3.1-9b-a2b` | 32K | $0.30 | $2.50 | text, image | 2026-10-15 |
| `moondream/moondream3-preview` | 32K | $0.30 | $2.50 | text, image | 2026-10-15 |

### morph (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `morph/morph-v3-large` | 262K | $0.90 | $1.90 | text | 2025-07-07 |
| `morph/morph-v3-fast` | 262K | $0.80 | $1.20 | text | 2025-07-07 |

### nex-agi (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `nex-agi/nex-n2.5-pro` | 262K | $0.075 | $0.25 | text, image | 2026-09-08 |
| `nex-agi/nex-n2.5-mini` | 262K | $0.025 | $0.100 | text, image | 2026-09-08 |

### nousresearch (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `nousresearch/hermes-4-405b` | 131K | $1.00 | $3.00 | text | 2025-08-26 |

### openrouter (4)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `openrouter/fusion` | 1M | $-1000000.000 | $-1000000.000 | text | 2026-06-13 |
| `openrouter/free` | 200K | free | free | text, image | 2026-01-31 |
| `openrouter/pareto-code` | 2M | $-1000000.000 | $-1000000.000 | text | 2026-04-21 |

<details><summary>Older openrouter versions and lines (1)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `openrouter/bodybuilder` | 128K | $-1000000.000 | $-1000000.000 | text | 2025-12-04 |

</details>

### particle-ai (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `particle-ai/particle-1` | 1.0M | $0.050 | $0.50 | text, image | 2026-10-28 |

### perceptron (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `perceptron/perceptron-mk1.5` | 37K | $0.15 | $1.50 | text, image, video, audio | 2026-09-25 |
| `perceptron/perceptron-mk1` | 33K | $0.15 | $1.50 | text, image, video | 2026-05-12 |

### perplexity (5)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `perplexity/sonar-pro-search` | 200K | $3.00 | $15.00 | text, image | 2025-10-30 |

<details><summary>Older perplexity versions and lines (4)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `perplexity/sonar-reasoning-pro` | 128K | $2.00 | $8.00 | text, image | 2025-03-06 |
| `perplexity/sonar-pro` | 200K | $3.00 | $15.00 | text, image | 2025-03-06 |
| `perplexity/sonar-deep-research` | 128K | $2.00 | $8.00 | text | 2025-03-06 |
| `perplexity/sonar` | 127K | $1.00 | $1.00 | text, image | 2025-01-27 |

</details>

### prism-ml (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `prism-ml/ternary-bonsai-2-27b` | 262K | $0.075 | $0.50 | text, image | 2026-09-18 |
| `prism-ml/ternary-bonsai-2-9b` | 262K | $0.080 | $0.13 | text, image | 2026-10-16 |

### recursive (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `recursive/flint-v1.0:free` | 262K | free | free | text | 2026-10-21 |

### reflection (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `reflection/beam-1.0-501b-a23b` | 1.0M | $0.30 | $1.00 | text | 2026-10-16 |

### rekaai (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `rekaai/reka-edge` | 16K | $0.100 | $0.100 | image, text, video | 2026-03-20 |
| `rekaai/reka-flash-3` | 66K | $0.100 | $0.20 | text | 2025-03-12 |

### relace (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `relace/relace-search` | 256K | $1.00 | $3.00 | text | 2025-12-08 |
| `relace/relace-apply-3` | 256K | $0.85 | $1.25 | text | 2025-09-26 |

### ricursiveai (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `ricursiveai/ripple-65b-preview` | 1.0M | $0.20 | $0.60 | text | 2026-11-04 |

### sakana (4)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `sakana/fugu-ultra-v2` | 1M | $5.00 | $30.00 | text, image, file | 2026-09-11 |
| `sakana/fugu-ultra` | 1M | $5.00 | $30.00 | text, image | 2026-06-24 |
| `sakana/fugu-max` | 1M | $2.00 | $6.00 | text, image, file | 2026-09-11 |
| `sakana/sakana-namazu` | 262K | $0.95 | $4.00 | text, image, file | 2026-08-10 |

### sao10k (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `sao10k/l3.1-euryale-70b` | 131K | $0.85 | $0.85 | text | 2024-08-27 |
| `sao10k/l3-lunaris-8b` | 8K | $0.040 | $0.050 | text | 2024-08-12 |

### scaledown (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `scaledown/summarize` | 1M | $0.040 | free | text, image, file | 2026-10-13 |
| `scaledown/extract` | 1M | $0.040 | free | text, image, file | 2026-10-13 |

### stealth (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `stealth/crouton-alpha` | 1.0M | free | free | text, image, video, file, audio | 2026-11-02 |

### stepfun (3)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `stepfun/step-3.7-flash` | 262K | $0.20 | $1.15 | text, image, video | 2026-05-28 |
| `stepfun/step-5-preview` | 1.0M | $1.00 | $2.70 | text, image, video | 2026-10-08 |
| `stepfun/step-3.5-flash` | 262K | $0.100 | $0.30 | text | 2026-01-29 |

### tencent (8)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `tencent/hy4` | 1.0M | $0.42 | $1.25 | text, image | 2026-11-03 |
| `tencent/hy4-preview` | 1.0M | $0.83 | $2.50 | text | 2026-08-28 |
| `tencent/hy-mt2-30b-a3b` | 8K | $0.074 | $0.29 | text | 2026-08-20 |
| `tencent/hy-mt2-1.8b` | 8K | $0.044 | $0.18 | text | 2026-08-20 |

<details><summary>Older tencent versions and lines (4)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `tencent/hy-mt2-7b` | 8K | $0.074 | $0.29 | text | 2026-08-19 |
| `tencent/hy3` | 262K | $0.13 | $0.53 | text | 2026-07-06 |
| `tencent/hy3-preview` | 262K | $0.18 | $0.60 | text | 2026-04-22 |
| `tencent/hunyuan-a13b-instruct` | 131K | $0.14 | $0.57 | text | 2025-07-08 |

</details>

### thedrummer (3)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `thedrummer/cydonia-24b-v4.1` | 131K | $0.30 | $0.50 | text | 2025-09-26 |
| `thedrummer/skyfall-36b-v2` | 33K | $0.55 | $0.80 | text | 2025-03-10 |
| `thedrummer/unslopnemo-12b` | 1.0M | $0.40 | $0.40 | text | 2024-11-08 |

### thinkingmachines (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `thinkingmachines/inkling` | 524K | $1.00 | $4.05 | text, image, audio | 2026-07-17 |
| `thinkingmachines/inkling-small` | 524K | $0.45 | $1.20 | text, image, audio | 2026-07-30 |

### typesafe (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `typesafe/jev-router` | 1M | $-1000000.000 | $-1000000.000 | audio, file, image, text, video | 2026-09-25 |

### unbiased (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `unbiased/pareto-26.9` | 262K | $2.50 | $7.50 | text, image | 2026-10-13 |
| `unbiased/pareto-26.10` | 1.0M | $0.80 | $3.20 | text, image | 2026-10-15 |

### undi95 (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `undi95/remm-slerp-l2-13b` | 6K | $0.35 | $0.65 | text | 2023-07-21 |

### upstage (2)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `upstage/solar-pro4` | 524K | $0.15 | $0.60 | text | 2026-08-10 |
| `upstage/solar-mini4` | 524K | $0.050 | $0.20 | text | 2026-09-23 |

### writer (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `writer/palmyra-x5` | 1.0M | $0.60 | $6.00 | text | 2026-01-21 |

### xiaomi (5)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `xiaomi/mimo-v2.6-pro-ultraspeed` | 1.0M | $4.35 | $8.70 | text, image, video, audio | 2026-09-21 |
| `xiaomi/mimo-v2.5` | 1M | $0.40 | $2.00 | text, audio, image, video | 2026-04-22 |
| `xiaomi/mimo-v2.6-pro` | 1.1M | $0.43 | $0.87 | text, image, video, audio | 2026-09-21 |
| `xiaomi/mimo-v2.6-flash` | 1.1M | $0.14 | $0.28 | text, image, video, audio | 2026-09-21 |

<details><summary>Older xiaomi versions and lines (1)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `xiaomi/mimo-v2.5-pro` | 1.1M | $0.48 | $1.80 | text | 2026-04-22 |

</details>

### xingchen-agi (1)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `xingchen-agi/xing4.0-29b-a4b:free` | 262K | free | free | text | 2026-10-16 |

## Video, audio and multimodal output

### openai (5)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `openai/gpt-5.4-image-2` | 272K | $8.00 | $15.00 | image, text, file | 2026-04-21 |
| `openai/gpt-audio` | 128K | $2.50 | $10.00 | text, audio | 2026-01-19 |
| `openai/gpt-audio-mini` | 128K | $0.60 | $2.40 | text, audio | 2026-01-19 |

<details><summary>Older openai versions and lines (2)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `openai/gpt-5-image-mini` | 400K | $2.50 | $2.00 | file, image, text | 2025-10-16 |
| `openai/gpt-5-image` | 400K | $10.00 | $10.00 | image, text, file | 2025-10-14 |

</details>

### google (11)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `google/gemini-3-pro-image` | 131K | $2.00 | $12.00 | image, text | 2026-06-17 |
| `google/gemini-omni-1.2-flash` | 131K | $1.50 | $9.00 | text, image, video | 2026-11-05 |
| `google/gemini-nano-banana-2.1` | 66K | $1.50 | $7.50 | image, text | 2026-10-06 |
| `google/gemini-3.1-flash-image` | 131K | $0.50 | $3.00 | image, text | 2026-06-17 |
| `google/gemini-3.1-flash-lite-image` | 66K | $0.25 | $1.50 | image, text | 2026-06-30 |

<details><summary>Older google versions and lines (6)</summary>

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| `google/gemini-omni-1.1-flash` | 131K | $1.50 | $9.00 | text, image, video | 2026-11-04 |
| `google/lyria-3-pro-preview` | 1.0M | free | free | text, image | 2026-03-30 |
| `google/lyria-3-clip-preview` | 1.0M | free | free | text, image | 2026-03-30 |
| `google/gemini-3.1-flash-image-preview` | 66K | $0.50 | $3.00 | image, text | 2026-02-26 |
| `google/gemini-3-pro-image-preview` | 66K | $2.00 | $12.00 | image, text | 2025-11-20 |
| `google/gemini-2.5-flash-image` | 33K | $0.30 | $2.50 | image, text | 2025-10-07 |

</details>

### openrouter (3)

| Model | Context | $/M in | $/M out | Input | Added |
|---|---|---|---|---|---|
| ⭐ `openrouter/auto` | 2M | $-1000000.000 | $-1000000.000 | text, image, audio, file, video | 2026-11-05 |
| `openrouter/auto-code` | 2M | $-1000000.000 | $-1000000.000 | text, image, audio, file, video | 2026-11-05 |
| `openrouter/auto-beta` | 2M | $-1000000.000 | $-1000000.000 | text, image, audio, file, video | 2026-07-17 |

## Image generation

### openai (10)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `openai/gpt-image-2.6-sunburst` | text, image | image | 2026-11-04 |
| `openai/gpt-image-2.6-flare` | text, image | image | 2026-11-04 |
| `openai/gpt-image-2.5-sunburst` | text, image | image | 2026-09-08 |

<details><summary>Older openai image models (7)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `openai/gpt-image-2.5-flare` | text, image | image | 2026-09-08 |
| `openai/gpt-image-2` | text, image | image | 2026-06-23 |
| `openai/gpt-image-1-mini` | text, image | image | 2026-06-23 |
| `openai/gpt-image-1` | text, image | image | 2026-06-23 |
| `openai/gpt-5.4-image-2` | image, text, file | image, text | 2026-04-21 |
| `openai/gpt-5-image-mini` | file, image, text | image, text | 2025-10-16 |
| `openai/gpt-5-image` | image, text, file | image, text | 2025-10-14 |

</details>

### google (7)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `google/gemini-nano-banana-2.1` | image, text | image, text | 2026-10-06 |
| `google/gemini-3.1-flash-lite-image` | image, text | image, text | 2026-06-30 |
| `google/gemini-3.1-flash-image` | image, text | image, text | 2026-06-17 |

<details><summary>Older google image models (4)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `google/gemini-3-pro-image` | image, text | image, text | 2026-06-17 |
| `google/gemini-3.1-flash-image-preview` | image, text | image, text | 2026-02-26 |
| `google/gemini-3-pro-image-preview` | image, text | image, text | 2025-11-20 |
| `google/gemini-2.5-flash-image` | image, text | image, text | 2025-10-07 |

</details>

### x-ai (1)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `x-ai/grok-imagine-image-2.0` | text, image | image | 2026-08-11 |

### meta (1)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `meta/muse-image` | text, image | image | 2026-08-26 |

### qwen (4)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `qwen/qwen-image-2.1-turbo` | text, image | image | 2026-10-16 |
| `qwen/qwen-image-2.1-pro` | text, image | image | 2026-10-16 |
| `qwen/qwen-image-3-pro` | text, image | image | 2026-08-04 |

<details><summary>Older qwen image models (1)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `qwen/qwen-image-3` | text, image | image | 2026-08-04 |

</details>

### microsoft (4)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `microsoft/mai-image-2.6` | text, image | image | 2026-09-04 |
| `microsoft/mai-image-2.6-flash` | text, image | image | 2026-09-04 |
| `microsoft/mai-image-2.5-pro` | text, image | image | 2026-07-23 |

<details><summary>Older microsoft image models (1)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `microsoft/mai-image-2.5` | text, image | image | 2026-06-02 |

</details>

### black-forest-labs (6)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `black-forest-labs/flux-3-image-klein` | text, image | image | 2026-10-15 |
| `black-forest-labs/flux-3-image` | text, image | image | 2026-10-01 |
| `black-forest-labs/flux.2-klein-4b` | text, image | image | 2026-01-14 |

<details><summary>Older black-forest-labs image models (3)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `black-forest-labs/flux.2-max` | text, image | image | 2025-12-15 |
| `black-forest-labs/flux.2-flex` | text, image | image | 2025-11-24 |
| `black-forest-labs/flux.2-pro` | text, image | image | 2025-11-24 |

</details>

### bytedance-seed (4)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `bytedance-seed/seedream-5-0-flash` | text, image | image | 2026-10-01 |
| `bytedance-seed/seedream-5-0-lite` | text, image | image | 2026-08-13 |
| `bytedance-seed/seedream-5-0-pro` | text, image | image | 2026-08-12 |

<details><summary>Older bytedance-seed image models (1)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `bytedance-seed/seedream-4.5` | image, text | image | 2025-12-23 |

</details>

### krea (4)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `krea/k3-edit` | text, image | image | 2026-10-15 |
| `krea/krea-2-large` | text, image | image | 2026-07-20 |
| `krea/krea-2-medium` | text, image | image | 2026-07-20 |

<details><summary>Older krea image models (1)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `krea/krea-2-medium-turbo` | text, image | image | 2026-07-20 |

</details>

### recraft (16)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `recraft/recraft-v4.1-flash` | text | image | 2026-09-23 |
| `recraft/recraft-v4-styles-pro` | text, image | image | 2026-08-26 |
| `recraft/recraft-v4-styles-vector` | text, image | image | 2026-08-26 |

<details><summary>Older recraft image models (13)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `recraft/recraft-v4-styles-pro-vector` | text, image | image | 2026-08-26 |
| `recraft/recraft-v4-styles` | text, image | image | 2026-08-26 |
| `recraft/recraft-v4.1-pro-vector` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-vector` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-utility-pro` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-utility` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1-pro` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4.1` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4-pro-vector` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4-vector` | text, image | image | 2026-05-13 |
| `recraft/recraft-v4-pro` | text, image | image | 2026-05-07 |
| `recraft/recraft-v4` | text, image | image | 2026-05-07 |
| `recraft/recraft-v3` | text, image | image | 2026-05-07 |

</details>

### sourceful (5)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `sourceful/riverflow-v2.6-pro` | text, image | image | 2026-10-29 |
| `sourceful/riverflow-v2.5-pro` | text, image | image | 2026-06-04 |
| `sourceful/riverflow-v2.5-fast` | text, image | image | 2026-06-04 |

<details><summary>Older sourceful image models (2)</summary>

| Model | Input | Output | Added |
|---|---|---|---|
| `sourceful/riverflow-v2-pro` | text, image | image | 2026-02-02 |
| `sourceful/riverflow-v2-fast` | text, image | image | 2026-02-02 |

</details>

### tencent (1)

| Model | Input | Output | Added |
|---|---|---|---|
| ⭐ `tencent/hy-image-v3.5-preview` | text, image | image | 2026-10-05 |

Image prices are per endpoint: `ab ai model <id>`.
