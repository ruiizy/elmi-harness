# elmiona

![Go](https://img.shields.io/badge/Go-1.21%2B-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-blue)

```
███████╗██╗     ███╗   ███╗██╗ ██████╗ ███╗   ██╗ █████╗
██╔════╝██║     ████╗ ████║██║██╔═══██╗████╗  ██║██╔══██╗
█████╗  ██║     ██╔████╔██║██║██║   ██║██╔██╗ ██║███████║
██╔══╝  ██║     ██║╚██╔╝██║██║██║   ██║██║╚██╗██║██╔══██║
███████╗███████╗██║ ╚═╝ ██║██║╚██████╔╝██║ ╚████║██║  ██║
╚══════╝╚══════╝╚═╝     ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝
```

A terminal-based AI coding assistant. Talk to Claude or any local model via Ollama, run shell commands, read and write files — all from an interactive REPL with a permission gate before anything destructive runs.

---

## Prerequisites

- **Go 1.21+**
- An **Anthropic API key** — if you want to use Claude models
- **[Ollama](https://ollama.com)** running locally — if you want local models

---

## Installation

```bash
git clone https://github.com/ruiizy/elmi-harness
cd elmi-harness
go build -o elmiona .
./elmiona
```

Or run directly without building:

```bash
go run .
```

---

## Configuration

Copy the example and fill in your keys:

```bash
cp env.example .env
```

| Variable            | Default                     | Description                                   |
|---------------------|-----------------------------|-----------------------------------------------|
| `ANTHROPIC_API_KEY` | —                           | Required when `DEFAULT_PROVIDER=anthropic`    |
| `DEFAULT_PROVIDER`  | `ollama`                    | `anthropic` or `ollama`                       |
| `DEFAULT_MODEL`     | `qwen3.5:9b`                | Model name to load on startup                 |
| `OLLAMA_BASE_URL`   | `http://localhost:11434`    | Ollama server address                         |

Environment variables override `.env` values.

---

## Usage

Start the REPL:

```bash
./elmiona
```

Type a message and press **Enter**. Use **↑ / ↓** to navigate your input history.

### Commands

| Command | Description |
|---------|-------------|
| `/help` | List all available commands |
| `/model` | Open the model picker (switch between Claude and Ollama models) |
| `/mode <subcommand>` | Change the tool permission mode (see below) |
| `/clear` | Clear the current conversation history |
| `/exit` | Exit elmiona |

### Permission Modes

Controls whether the agent needs your approval before running tools.

```
/mode always-ask              # ask before every tool call  (default)
/mode always-allow            # never ask — run everything automatically
/mode ask-once                # ask once per tool per session, auto after
/mode allow-list bash         # auto-run bash only; ask for everything else
/mode allow-list bash read_file write_file   # multiple tools
```

---

## Tools

The agent has three built-in tools:

| Tool | Description | Limits |
|------|-------------|--------|
| `bash` | Run a shell command, returns stdout + stderr | 120 s timeout |
| `read_file` | Read a file at the given path | 10 MB max |
| `write_file` | Write or overwrite a file | — |

---

## Providers

### Claude (Anthropic)

Set `DEFAULT_PROVIDER=anthropic` and provide `ANTHROPIC_API_KEY`. Available models:

- `claude-opus-4-7-20250514`
- `claude-sonnet-4-6`
- `claude-haiku-4-5-20251001`

### Local models (Ollama)

Start Ollama and pull a model:

```bash
ollama pull qwen2.5-coder:7b
```

elmiona fetches the model list from the Ollama API automatically — they appear in `/model` picker under **Ollama (local)**.

---

## Project structure

```
.
├── main.go                  # REPL loop, command registration
├── commands.go              # Command registry and dispatcher
├── config/                  # .env + env var loading
├── internal/
│   ├── agent/               # LLM agent loop (stream → tool calls → repeat)
│   ├── api/                 # Shared message and tool types
│   ├── perm/                # Permission mode state machine
│   └── tools/               # Tool implementations (bash, read_file, write_file)
├── provider/
│   ├── anthropic/           # Anthropic SDK adapter
│   └── openai/              # OpenAI-compatible adapter (Ollama, LM Studio)
└── ui/                      # Terminal UI (banner, spinner, model picker, confirm)
```
