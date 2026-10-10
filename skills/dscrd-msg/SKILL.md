---
name: dscrd-msg
description: "Read and send messages"
metadata:
  version: 0.2.0
  openclaw:
    category: "productivity"
    requires:
      bins:
        - dscrd
    cliHelp: "dscrd msg --help"
---

# dscrd msg

Read and send messages

> **PREREQUISITE:** Read `../dscrd-shared/SKILL.md` for auth, global flags, security rules, and exit codes. If missing, run `dscrd generate-skills`.

| Command | Description |
|---------|-------------|
| `dscrd msg delete` | Delete a message |
| `dscrd msg edit` | Edit a message the bot sent |
| `dscrd msg permalink` | Print the web URL of a message |
| `dscrd msg pin` | Pin a message |
| `dscrd msg pins` | List pinned messages |
| `dscrd msg react` | Add a reaction (unicode emoji or custom name:id) |
| `dscrd msg reactions` | List users who reacted with an emoji |
| `dscrd msg read` | Read recent messages in a channel |
| `dscrd msg send` | Send a message (optionally as a reply or with a file) |
| `dscrd msg unpin` | Unpin a message |
| `dscrd msg unreact` | Remove the bot's reaction |

## dscrd msg delete

Delete a message

**Discord API:** `DELETE /channels/{channel.id}/messages/{message.id}`

**Bot permissions:** `MANAGE_MESSAGES`

```bash
dscrd msg delete [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--message` | ✓ | — | message ID |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd msg edit

Edit a message the bot sent

**Discord API:** `PATCH /channels/{channel.id}/messages/{message.id}`

```bash
dscrd msg edit [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--message` | ✓ | — | message ID |
| `--text` | — | — | new text |
| `--text-file` | — | — | path to text file (use - for stdin) |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd msg permalink

Print the web URL of a message

```bash
dscrd msg permalink [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--message` | ✓ | — | message ID |

## dscrd msg pin

Pin a message

**Discord API:** `PUT /channels/{channel.id}/pins/{message.id}`

**Bot permissions:** `MANAGE_MESSAGES`

```bash
dscrd msg pin [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--message` | ✓ | — | message ID |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd msg pins

List pinned messages

**Discord API:** `GET /channels/{channel.id}/pins`

**Privileged intents:** `MESSAGE_CONTENT` (enable in Developer Portal -> Bot -> Privileged Gateway Intents)

```bash
dscrd msg pins [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |

## dscrd msg react

Add a reaction (unicode emoji or custom name:id)

**Discord API:** `PUT /channels/{channel.id}/messages/{message.id}/reactions/{emoji}/@me`

**Bot permissions:** `ADD_REACTIONS`

```bash
dscrd msg react [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--emoji` | ✓ | — | emoji (unicode or custom name:id) |
| `--message` | ✓ | — | message ID |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd msg reactions

List users who reacted with an emoji

**Discord API:** `GET /channels/{channel.id}/messages/{message.id}/reactions/{emoji}`

```bash
dscrd msg reactions [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--emoji` | ✓ | — | emoji (unicode or custom name:id) |
| `--message` | ✓ | — | message ID |

## dscrd msg read

Read recent messages in a channel

**Discord API:** `GET /channels/{channel.id}/messages`

**Bot permissions:** `VIEW_CHANNEL,READ_MESSAGE_HISTORY`

**Privileged intents:** `MESSAGE_CONTENT` (enable in Developer Portal -> Bot -> Privileged Gateway Intents)

```bash
dscrd msg read [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--after` | — | — | only messages after this message ID |
| `--around` | — | — | messages around this message ID |
| `--before` | — | — | only messages before this message ID |
| `--channel` | ✓ | — | channel ID or name |
| `--limit` | — | `5` | max messages |

## dscrd msg send

Send a message (optionally as a reply or with a file)

**Discord API:** `POST /channels/{channel.id}/messages`

**Bot permissions:** `SEND_MESSAGES`

```bash
dscrd msg send [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--allow-mentions` | — | — | let @everyone/@here, role and user mentions in the text ping (default: suppressed) |
| `--channel` | ✓ | — | channel ID or name |
| `--file` | — | — | path of a file to attach |
| `--reply-to` | — | — | message ID to reply to |
| `--text` | — | — | message text |
| `--text-file` | — | — | path to text file (use - for stdin) |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd msg unpin

Unpin a message

**Discord API:** `DELETE /channels/{channel.id}/pins/{message.id}`

**Bot permissions:** `MANAGE_MESSAGES`

```bash
dscrd msg unpin [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--message` | ✓ | — | message ID |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd msg unreact

Remove the bot's reaction

**Discord API:** `DELETE /channels/{channel.id}/messages/{message.id}/reactions/{emoji}/@me`

```bash
dscrd msg unreact [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--emoji` | ✓ | — | emoji (unicode or custom name:id) |
| `--message` | ✓ | — | message ID |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.


