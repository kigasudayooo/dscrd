---
name: dscrd-webhook
description: "Manage webhooks"
metadata:
  version: 0.2.0
  openclaw:
    category: "productivity"
    requires:
      bins:
        - dscrd
    cliHelp: "dscrd webhook --help"
---

# dscrd webhook

Manage webhooks

> **PREREQUISITE:** Read `../dscrd-shared/SKILL.md` for auth, global flags, security rules, and exit codes. If missing, run `dscrd generate-skills`.

| Command | Description |
|---------|-------------|
| `dscrd webhook create` | Create a webhook on a channel |
| `dscrd webhook delete` | Delete a webhook |
| `dscrd webhook execute` | Post a message through a webhook |
| `dscrd webhook list` | List webhooks for the server or one channel |

## dscrd webhook create

Create a webhook on a channel

**Discord API:** `POST /channels/{channel.id}/webhooks`

**Bot permissions:** `MANAGE_WEBHOOKS`

```bash
dscrd webhook create [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | ✓ | — | channel ID or name |
| `--name` | ✓ | — | webhook name |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd webhook delete

Delete a webhook

**Discord API:** `DELETE /webhooks/{webhook.id}`

**Bot permissions:** `MANAGE_WEBHOOKS`

```bash
dscrd webhook delete <webhook-id>
```

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd webhook execute

Post a message through a webhook

**Discord API:** `POST /webhooks/{webhook.id}/{webhook.token}`

```bash
dscrd webhook execute [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--allow-mentions` | — | — | let @everyone/@here, role and user mentions in the text ping (default: suppressed) |
| `--id` | ✓ | — | webhook ID |
| `--text` | — | — | message text |
| `--text-file` | — | — | path to text file (use - for stdin) |
| `--token` | ✓ | — | webhook token |

> [!CAUTION]
> Write command — confirm with the user before executing; preview with `--dry-run`.

## dscrd webhook list

List webhooks for the server or one channel

**Discord API:** `GET /guilds/{guild.id}/webhooks`

**Bot permissions:** `MANAGE_WEBHOOKS`

```bash
dscrd webhook list [flags]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--channel` | — | — | restrict to one channel (ID or name) |


