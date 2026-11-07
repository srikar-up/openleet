# AI Providers

OpenLeet's Socratic Coach connects to a local Ollama instance or a cloud LLM provider. Provider credentials are stored in the encrypted vault, never in `config.json`.

## Supported providers

| Provider | ID | Protocol | Default endpoint | Default model | Key prefix |
| :--- | :--- | :--- | :--- | :--- | :--- |
| OpenCode Console | `opencode` | OpenAI-compatible | `https://opencode.ai/inference/v1` | `deepseek-v4.1-flash-free` | `sk-` |
| OpenAI | `openai` | OpenAI-compatible | `https://api.openai.com` | `gpt-4.1` | `sk-` |
| Groq | `groq` | OpenAI-compatible | `https://api.groq.com/openai/v1` | `openai/gpt-oss-120b` | `gsk_` |
| Anthropic | `anthropic` | Anthropic Messages | `https://api.anthropic.com` | `claude-sonnet-4-6` | `sk-ant-` |
| Google Gemini | `gemini` | Gemini generateContent | `https://generativelanguage.googleapis.com` | `gemini-3.8-flash` | `AIza` |
| NVIDIA NIM | `nvidia` | OpenAI-compatible | `https://integrate.api.nvidia.com/v1` | `nvidia/nemotron-3-super-120b-a12b` | `nvapi-` |
| OpenRouter | `openrouter` | OpenAI-compatible | `https://openrouter.ai/api` | `openrouter/auto` | `sk-or-` |
| Ollama | `ollama` | Local native | `http://localhost:11434` | `qwen2.5-coder:7b` | none |
| Custom | `custom` | OpenAI-compatible | user-supplied | user-supplied | `sk-` |

## Connect a provider

1. Open **Settings → AI Models**.
2. Press `c` or click **Connect an integration**.
3. Search for a provider.
4. Press `Enter`.
5. Enter the API key.
6. Optionally override the base URL and model in the provider detail card.

The selected provider is stored in `ai.coach`. Per-provider overrides use:

```text
ai.provider.<id>.base_url
ai.provider.<id>.model
```

## Selection behavior

### Explicit provider

If `ai.coach` names a provider, that provider is used directly.

- An explicit cloud provider does **not** silently fall back to Ollama.
- If the key is missing, the coach reports that no provider is available.
- Selecting Ollama keeps execution local and does not require a key.

### Auto

When `ai.coach` is `auto`:

1. Ollama is tried first.
2. If Ollama is unavailable, the first saved cloud key wins.
3. Priority order: `opencode`, `openai`, `groq`, `anthropic`, `gemini`, `nvidia`, `openrouter`, then the remaining providers.
4. If no vault key exists, the legacy `OPENCODE_API_KEY` / `pass.env` path is used.

## Security

- Keys are stored in `secrets.enc` using AES-256-GCM.
- The vault file is mode `0600`.
- The key is derived from the current OS user and host; it is not compiled into the binary.
- Keys are masked in provider cards and picker rows.
- Firebase ID tokens are memory-only.
- Sync metadata contains only UID, timestamp, and a short status note.

## Live endpoint examples

These are intentionally minimal checks. Use your own keys and never commit them.

### Google Gemini

```bash
curl -sS \
  "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:streamGenerateContent?alt=sse&key=$GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "systemInstruction": {"parts": [{"text": "You are a concise DSA interview coach."}]},
    "contents": [{"role": "user", "parts": [{"text": "Give me one hint for Two Sum."}]}],
    "generationConfig": {"temperature": 0.2}
  }'
```

### NVIDIA NIM

```bash
curl -sS \
  "https://integrate.api.nvidia.com/v1/chat/completions" \
  -H "Authorization: Bearer $NVIDIA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nvidia/nemotron-3-super-120b-a12b",
    "messages": [{"role": "user", "content": "Give me one hint for Two Sum."}],
    "stream": true
  }'
```

### Groq

```bash
curl -sS \
  "https://api.groq.com/openai/v1/chat/completions" \
  -H "Authorization: Bearer $GROQ_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-oss-120b",
    "messages": [{"role": "user", "content": "Give me one hint for Two Sum."}],
    "stream": true
  }'
```

### Anthropic

```bash
curl -sS \
  "https://api.anthropic.com/v1/messages" \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 512,
    "system": "You are a concise DSA interview coach.",
    "messages": [{"role": "user", "content": "Give me one hint for Two Sum."}],
    "stream": true
  }'
```

### OpenAI-compatible

```bash
curl -sS \
  "$OPENAI_BASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4.1",
    "messages": [{"role": "user", "content": "Give me one hint for Two Sum."}],
    "stream": true
  }'
```

## Troubleshooting

| Symptom | Likely cause | Fix |
| :--- | :--- | :--- |
| `no provider available` | No key saved, or explicit provider has no key | Open the integration picker and save a key |
| `401` / `403` | Invalid key or provider permission | Re-enter the key and test the provider |
| `404` | Base URL or model name is wrong | Check the provider detail card; use the default endpoint |
| `404` `Function '…': Not found for account '…'` from NVIDIA | The key is valid and `/models` works, but the NVIDIA account lacks **Public API Endpoints** access | Enable it at `build.nvidia.com` or email `help@build.nvidia.com`; model or key changes cannot bypass it |
| Gemini says the model is “no longer available to new users” | An old 2.5 Pro/Flash/Flash-Lite ID is pinned | Select `gemini-3.8-flash`; the migration also rewrites retired 2.5 pins automatically |
| Gemini `503 UNAVAILABLE` | 3.8 Flash is temporarily over capacity | Retry shortly; the app now reports this as a transient overload, not a bad key |
| Gemini returns HTML | Missing `alt=sse` or wrong endpoint | Use the `/v1beta/models/...:streamGenerateContent?alt=sse` path |
| NVIDIA returns 404 for a model | The model ID is unavailable for this account | Override the model with one available in your NVIDIA account |
| Ollama ignored | An explicit cloud provider is selected | Set `ai.coach` to `auto` or `ollama` |

## Provider probe

Press `t` in a provider detail card to run a connectivity probe. A saved key by itself is shown as `key saved (untested)`; only a successful probe reports `connected in <latency>`, and a failed probe shows `test failed`.

The probe uses:

- Ollama: `GET /api/tags`
- Gemini: `GET /v1beta/models?key=...`, then a one-token `POST ...:generateContent` for the selected model
- Anthropic: `GET /v1/models` with `x-api-key` and `anthropic-version`
- OpenAI-compatible: `GET /v1/models` with a Bearer token, then a one-token `POST /v1/chat/completions` to prove generation works
