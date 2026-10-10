---
name: dscrd-thread
description: "Work with threads and forum posts"
metadata:
  version: 0.2.0
  openclaw:
    category: "productivity"
    requires:
      bins:
        - dscrd
    cliHelp: "dscrd thread --help"
---

# dscrd thread

Work with threads and forum posts

> **PREREQUISITE:** Read `../dscrd-shared/SKILL.md` for auth, global flags, security rules, and exit codes. If missing, run `dscrd generate-skills`.

| Command | Description |
|---------|-------------|
| `dscrd thread archive` | Archive a thread |
| `dscrd thread create` | Create a thread (from a message, standalone, or a forum post with --text) |
| `dscrd thread list` | List active threads in the server (or archived ones in a channel) |
| `dscrd thread read` | Read messages in a thread |
| `dscrd thread reply` | Reply in a thread |

## dscrd thread archive

Archive a thread

**Discord API:** `PATCH /channels/{thread.id}`

**Bot permissions:** `MANAGE_THREADS`

```bash
dscrd thread archive [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--thread` | ✓ | — | thread ID |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd thread create

Create a thread (from a message, standalone, or a forum post with --text)

**Discord API:** `POST /channels/{channel.id}/threads`

**Bot permissions:** `CREATE_PUBLIC_THREADS`

```bash
dscrd thread create [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--allow-mentions` | — | — | let @everyone/@here, role and user mentions in the text ping (default: suppressed) |
| `--channel` | ✓ | — | parent channel ID or name |
| `--from-message` | — | — | start the thread from this message ID |
| `--name` | ✓ | — | thread name |
| `--text` | — | — | initial message (required for forum channels) |
| `--text-file` | — | — | path to initial message file (use - for stdin) |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd thread list

List active threads in the server (or archived ones in a channel)

**Discord API:** `GET /guilds/{guild.id}/threads/active`

```bash
dscrd thread list [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--archived` | — | — | list archived public threads of --channel |
| `--channel` | — | — | channel ID or name (for --archived) |

## dscrd thread read

Read messages in a thread

**Discord API:** `GET /channels/{thread.id}/messages`

**Privileged intents:** `MESSAGE_CONTENT` (enable in Developer Portal -> Bot -> Privileged Gateway Intents)

```bash
dscrd thread read [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--limit` | — | `10` | max messages |
| `--thread` | ✓ | — | thread ID |

## dscrd thread reply

Reply in a thread

**Discord API:** `POST /channels/{thread.id}/messages`

**Bot permissions:** `SEND_MESSAGES_IN_THREADS`

```bash
dscrd thread reply [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--allow-mentions` | — | — | let @everyone/@here, role and user mentions in the text ping (default: suppressed) |
| `--text` | — | — | reply text |
| `--text-file` | — | — | path to text file (use - for stdin) |
| `--thread` | ✓ | — | thread ID |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.


