# ctxpack

**Task-aware context bundler for AI coding tools**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

You open a new AI chat. You paste in your project structure. You explain what the codebase does. Again. You do this every single session because the AI has no memory of what you built or why.

`ctxpack` fixes this. Give it a task description; it scans your repo, ranks every file by relevance using TF-IDF, and bundles the top matches into a ready-to-paste context block — within a ~4 000-token budget.

---

## Install

```sh
go install github.com/ctxpackhq/ctxpack@latest
```

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
Token estimate: 3821 / 4000
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
ctxpack "fix the auth bug" -d ~/projects/backend
```

**Pipe to a file instead of clipboard:**

```sh
ctxpack "refactor database layer" > context.md
```

---

## How it works

- Walks your repo, skipping `node_modules`, `.git`, `vendor`, lock files, and binaries.
- Scores every file against your task description with TF-IDF — files whose content and path share the most terms with your task rank highest.
- Greedily packs the top-scoring files into a single Markdown block, stopping at ~4 000 tokens.

---

## License

MIT
