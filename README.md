# dscrd

<!-- Status -->
[![CI](https://img.shields.io/github/actions/workflow/status/howar31/dscrd/ci.yml?branch=main&label=CI)](https://github.com/howar31/dscrd/actions/workflows/ci.yml)
[![Go 1.25+](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/conventional%20commits-1.0.0-yellow)](https://www.conventionalcommits.org)
[![Dependabot](https://img.shields.io/badge/dependabot-enabled-025E8C?logo=dependabot)](.github/dependabot.yml)

<!-- Release & distribution -->
[![GitHub release](https://img.shields.io/github/v/release/howar31/dscrd)](https://github.com/howar31/dscrd/releases)
[![GitHub release downloads](https://img.shields.io/github/downloads/howar31/dscrd/total?label=release%20downloads)](https://github.com/howar31/dscrd/releases)
[![npm version](https://img.shields.io/npm/v/@howar31/dscrd)](https://www.npmjs.com/package/@howar31/dscrd)
[![npm downloads](https://img.shields.io/npm/dm/@howar31/dscrd?label=npm%20downloads)](https://www.npmjs.com/package/@howar31/dscrd)

<!-- Activity & community -->
[![Last commit](https://img.shields.io/github/last-commit/howar31/dscrd)](https://github.com/howar31/dscrd/commits/main)
[![Open issues](https://img.shields.io/github/issues/howar31/dscrd)](https://github.com/howar31/dscrd/issues)
[![Stars](https://img.shields.io/github/stars/howar31/dscrd)](https://github.com/howar31/dscrd/stargazers)
[![Sponsor](https://img.shields.io/badge/Sponsor-donate.howar31.com-b4532c?logo=data:image/svg%2Bxml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZmlsbD0iI2ZmZiIgZD0iTTIwLjg0IDQuNjFhNS41IDUuNSAwIDAgMC03Ljc4IDBMMTIgNS42N2wtMS4wNi0xLjA2YTUuNSA1LjUgMCAwIDAtNy43OCA3Ljc4bDEuMDYgMS4wNkwxMiAyMS4yM2w3Ljc4LTcuNzggMS4wNi0xLjA2YTUuNSA1LjUgMCAwIDAgMC03Ljc4eiIvPjwvc3ZnPg==)](https://donate.howar31.com/)

**Agent-facing Discord CLI.** One command = one REST call, token-efficient output, no gateway connection, no daemon. Built for AI coding agents (and humans who live in the terminal) to read and operate Discord servers through a bot.

```console
$ dscrd msg read --channel "#general" --limit 3
Alice: anyone up for a review? [07-03 14:02] (333333333333333333)
Bob: on it 👀 [07-03 14:05] (333333333333333334)
Alice: thanks! [07-03 14:06] (333333333333333335)

$ dscrd msg send --channel "#general" --text "deploy finished" --reply-to 333333333333333334
sent 333333333333333336
```

## Why

- **Token-efficient by default.** The `concise` format renders one line per item — a message read costs an agent a few dozen tokens instead of a full JSON envelope. `--raw` / `--format json` are one flag away when structure is needed.
- **Stateless one-shot calls.** Every invocation is a single HTTP round-trip (plus automatic rate-limit retries). The bot never needs to be "online".
- **Full API reach.** Common operations are wrapped as commands; everything else is reachable via `dscrd api <METHOD> <path>`.
- **Guardrails built in.** Bot tokens are AES-256-GCM-encrypted at rest, write commands support `--dry-run`, and errors carry actionable hints (missing permission? missing intent? the message tells you).

## What a bot can and cannot do

dscrd operates as a **Discord bot** — the only automation model Discord's Terms of Service permit. That shapes the scope honestly:

| Works | Does not exist for bots |
|---|---|
| Read/send/edit messages, threads, forums | Acting as your personal account |
| Per-server message search | Cross-server or DM search |
| Channels, roles, members, invites, webhooks | Seeing servers the bot wasn't invited to |
| DMs as the bot (mutual server required) | Reading your personal DM inbox |
| Audit logs, scheduled events, polls, automod | Presence / online status (gateway-only) |

## Install

```bash
# npm (downloads the platform binary, verifies SHA256)
npm install -g @howar31/dscrd

# or Homebrew (macOS)
brew install howar31/tap/dscrd

# or Go
go install github.com/howar31/dscrd/cmd/dscrd@latest

# or grab a binary from GitHub Releases
```

## Bot setup (one-time, ~3 minutes)

1. Create an application at <https://discord.com/developers/applications> → **New Application**.
2. Under **Bot**: copy the **Token**, and enable the Privileged Gateway Intents you need:
   - **MESSAGE CONTENT INTENT** — required to read message text and search.
   - **SERVER MEMBERS INTENT** — required for `member list`.
3. Store the credentials (the token is encrypted at rest). `--name` labels the
   profile — pick something you will recognise later, because it is how you
   select this bot:

   ```bash
   dscrd auth set-token --name <label> --token <bot-token> --application-id <application-id>
   ```

   Omitting `--name` stores the profile as `default`, which gets confusing the
   moment you add a second bot. Already did? `dscrd auth rename default <label>`.

4. Invite the bot to your server (requires Manage Server on that server):

   ```bash
   dscrd auth invite-url                  # sensible read+write preset
   dscrd auth invite-url --permissions read
   ```

   Open the printed URL, pick the server, done.
5. Verify:

   ```bash
   dscrd auth test        # -> ok yourbot (1234...)
   dscrd guild list
   ```

Set a default server so you can skip `--guild` on every call — easiest to pass
it during step 3, alongside the token:

```bash
dscrd auth set-token --name <label> --token <bot-token> --default-guild <guild-id>
```

Updating a profile that already exists needs `--force`, which confirms you mean
to replace its stored token:

```bash
dscrd auth set-token --name <label> --token <bot-token> --default-guild <guild-id> --force
```

### Multiple bots

Each bot is a named profile. Tokens cannot be read back, so `set-token` refuses
to overwrite an existing profile unless you pass `--force`.

```bash
dscrd auth status                     # list profiles; * marks the active one
dscrd auth switch <label>             # change the active profile
dscrd auth rename <old> <new>         # relabel a profile (active follows it)
dscrd auth logout --name <label>      # remove one

dscrd --profile <label> msg read --channel "#general"   # one-off, no switch
```

Profile names take letters, digits, `-` and `_`. Every write command above
accepts `--dry-run` to preview without touching the config.

## Command tour

```text
dscrd guild    list|info                          servers the bot is in
dscrd channel  list|info|create|edit|delete|topic
dscrd msg      read|send|edit|delete|react|unreact|reactions|pin|unpin|pins|permalink
dscrd thread   list|create|read|reply|archive     (forum posts = thread create --text)
dscrd search   messages                           per-server full-text search
dscrd member   list|info|search
dscrd role     list|create|edit|delete|assign|unassign
dscrd dm       send|read                          as the bot; mutual server required
dscrd emoji    list|info                          dscrd sticker list|info
dscrd file     download                           attachments via msg send --file
dscrd invite   list|create|delete|info            dscrd webhook list|create|delete|execute
dscrd event    list|create|edit|delete            dscrd audit-log read
dscrd poll     create|results|end                 dscrd automod list|info|create|delete
dscrd api      <METHOD> <path>                    any REST endpoint
```

`--help` on any level gives the full reference. Channels accept IDs or names (`--channel "#general"`); the target server comes from `--guild <id|name>` or the profile's `default_guild`.

### Global flags

| Flag | Meaning |
|---|---|
| `--format concise\|json\|jsonl\|table` | output format (default `concise`) |
| `--guild <id\|name>` | target server |
| `--profile <name>` | which stored bot to use |
| `--raw` | print the raw Discord JSON response |
| `--dry-run` | print the would-be API call, send nothing |
| `--no-resolve` | skip ID→name prettifying |

### Exit codes

`0` ok · `3` auth · `4` not found · `5` rate-limited · `1` other.

## For AI agents

The `skills/` directory contains a generated skill tree (an index, a shared reference, and one skill per command group) that teaches an agent the full command surface with per-verb API routes, permission requirements, and write-operation cautions. It is generated from the live command tree (`dscrd generate-skills`) and CI fails if it drifts.

```bash
npx skills add https://github.com/howar31/dscrd
```

## Security

- Tokens are stored AES-256-GCM-encrypted in `~/.config/dscrd/config.toml` (mode 0600); the key lives in the OS keychain, or a 0600 file when no keychain is available (`DSCRD_KEYRING_BACKEND=auto|keyring|file`).
- Tokens are never printed or logged. `auth status` reports state (`set`/`locked`/`empty`), never values.
- `DSCRD_TOKEN` (env) overrides stored profiles for ephemeral use.
- `file download` only fetches from Discord CDN hosts over https, and refuses to overwrite an existing file unless `--force` is passed.
- Messages the bot posts do not ping: `@everyone`/`@here`, role and user mentions are suppressed unless you pass `--allow-mentions`.
- ID flags (`--message`, `--user`, `--role`, …) accept Discord IDs only; message text is printed on one line with line breaks shown as `\n`.

See [SECURITY.md](SECURITY.md) for reporting.

## Development

```bash
go build -o dscrd ./cmd/dscrd     # version comes from the VERSION file
go test ./...                     # includes the command-coverage meta-test
go run ./cmd/dscrd generate-skills
scripts/smoke.sh                  # live test against a real server (env-gated)
```

Every leaf command must be exercised by at least one test — a meta-test walks the command tree and fails CI when coverage is missing.

Releases are driven by the `VERSION` file: bumping it on `main` tags `v<VERSION>`, builds binaries via goreleaser, and publishes `@howar31/dscrd`.

## License

[MIT](LICENSE)
