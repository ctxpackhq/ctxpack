# ctxpack

**Task-aware context bundler for AI coding tools**

[![Demo](https://asciinema.org/a/VNvDN6vhL06FNeO5.svg)](https://asciinema.org/a/VNvDN6vhL06FNeO5)

[![License: BUSL-1.1](https://img.shields.io/badge/License-BUSL--1.1-orange.svg)](https://mariadb.com/bsl11/)

---

You open a new AI chat. You paste in your project structure. You explain what the codebase does. Again. You do this every single session because the AI has no memory of what you built or why.

`ctxpack` fixes this. Give it a task description; it scans your repo, ranks every file by relevance using TF-IDF, and bundles the top matches into a ready-to-paste context block — within a ~8 000-token budget.

---

## Install

**Go install:**
```sh
go install github.com/ctxpackhq/ctxpack@v0.1.10
```

**Pre-built binaries (no Go required):**

| Platform | Download |
|---|---|
| Linux amd64 | [ctxpack-linux-amd64](https://github.com/ctxpackhq/ctxpack/releases/download/v0.1.10/ctxpack-linux-amd64) |
| macOS amd64 | [ctxpack-darwin-amd64](https://github.com/ctxpackhq/ctxpack/releases/download/v0.1.10/ctxpack-darwin-amd64) |
| macOS arm64 (M1/M2) | [ctxpack-darwin-arm64](https://github.com/ctxpackhq/ctxpack/releases/download/v0.1.10/ctxpack-darwin-arm64) |
| Windows amd64 | [ctxpack-windows-amd64.exe](https://github.com/ctxpackhq/ctxpack/releases/download/v0.1.10/ctxpack-windows-amd64.exe) |

---

## Usage

```sh
ctxpack "add rate limiting to the API handler"
```

```
Scanning /home/user/myapp...
Scoring 43 files...
Copied to clipboard.

--- Summary ---
Files scanned:  43
Files selected: 6
Token estimate: 3821 / 8000
```

The formatted context is written to stdout and copied to your clipboard:

```markdown
# Context for: add rate limiting to the API handler

## internal/handler/api.go

​```
package handler
...
​```

## internal/middleware/middleware.go

​```
package middleware
...
​```
```

Paste it directly into Claude, ChatGPT, or any chat interface — no manual file hunting.

**Scan a different directory:**

```sh
ctxpack "fix the auth bug" --dir ~/projects/backend
```

**Pipe to a file instead of clipboard:**

```sh
ctxpack "refactor database layer" > context.md
```

**Preview scored files before committing to a paste:**

```sh
ctxpack "add rate limiting to the API handler" --preview
```

```
┌─────────────────────────────┬───────┬────────┐
│ File                        │ Score │ Tokens │
├─────────────────────────────┼───────┼────────┤
│ internal/handler/api.go     │ 3.42  │    821 │
│ internal/middleware/auth.go │ 2.18  │    634 │
│ internal/router/router.go   │ 1.05  │    312 │
└─────────────────────────────┴───────┴────────┘
Total: 3 files, 1767 tok
```

No output is written to stdout or clipboard in preview mode.

---

## Flags

| Flag | Default | Description |
|---|---|---|
| `-d, --dir` | `.` | Root directory to scan |
| `--max-tokens` | `8000` | Token budget for selected files |
| `--no-clipboard` | `false` | Write to stdout only; skip clipboard |
| `--preview` | `false` | Show scored file table; no output |

---

## How it works

- Walks your repo, skipping `node_modules`, `.git`, `vendor`, lock files, and binaries.
- Scores every file against your task description with TF-IDF — files whose content and path share the most terms with your task rank highest.
- Greedily packs the top-scoring files into a single Markdown block, stopping at ~8 000 tokens.

---

## Security

- `.env`, `.env.local`, and all `.env.*` files are never included in output.
- `.gitignore` is respected — any file or directory your repo ignores, ctxpack ignores too.

---

## License

[Business Source License 1.1](LICENSE)
