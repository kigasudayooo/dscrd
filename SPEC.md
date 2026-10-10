# dscrd — SPEC

## Purpose

`dscrd` is an agent-facing Discord CLI: a single static Go binary that lets AI coding agents (and terminal users) read and operate Discord servers through a bot account. One command equals one REST call — no gateway websocket, no daemon, no persistent process. Output defaults to a token-efficient one-line-per-item format so LLM agents pay minimal context cost per call; full JSON is opt-in.

The tool is bot-token only by design: Discord's Terms of Service forbid automating personal user accounts, so the CLI's honest scope is "operations on servers the bot has been invited to", never "acting as the user".

## Architecture

Layered single binary, stateless per invocation:

1. **`cmd/dscrd/main.go`** — builds the Cobra root command, executes it, and maps typed errors to exit codes (`*api.APIError`, `*auth.AuthError`).
2. **`internal/commands`** — the command tree: one file per command group (21 groups, ~78 leaf verbs). Each leaf resolves credentials, builds the request, calls the API client, and renders items. Cobra `Annotations` carry per-command metadata (`discordEndpoint`, `write`, `perms`, `intents`) that drives both runtime behavior (dry-run cautioning) and skill generation.
3. **`internal/api`** — hand-written REST client for `https://discord.com/api/v10`. `Do(method, path, body)` sends JSON; `DoMultipart` uploads files (`payload_json` + `files[0]`). Automatic retries: HTTP 429 (rate limit) and the message-search index warm-up response (HTTP 202 + error code `110000`), both honoring `retry_after`. Non-2xx responses become `APIError{Status, Code, Message, Hint}`; a hint table maps common Discord error codes (50001/50013/50007/110000) to actionable guidance (which permission to grant, which Developer Portal intent to enable).
4. **`internal/auth`** — profile store at `~/.config/dscrd/config.toml` (mode 0600, TOML). Bot tokens are encrypted at rest with AES-256-GCM; the key lives in the OS keyring (service `dscrd`) or a 0600 key file, selected by `DSCRD_KEYRING_BACKEND` (`auto`/`keyring`/`file`). Token resolution precedence: `DSCRD_TOKEN` env → `--profile` flag → `DSCRD_PROFILE` env → config `active` profile. A profile also carries `application_id` (for invite-URL generation) and `default_guild`. Profile names are validated as `[A-Za-z0-9_-]+` on write only (`ValidateProfileName`), keeping them usable as bare TOML keys without invalidating configs written before the rule. Because a stored token cannot be read back, destructive profile writes are opt-in: `auth set-token` and `auth rename` refuse to clobber an existing profile without `--force`.
5. **`internal/output`** — `Emit(w, format, items)` renders any slice in `concise` (via the `Concise() string` interface), `json`, `jsonl`, or `table` format.
6. **`internal/resolve`** — best-effort ID→name cache (`~/.config/dscrd/cache/resolve.json`), keyed `kind:id`, used e.g. to prettify `<@id>`/`<#id>` mentions in message output. Lookup failures degrade to the raw ID. `--no-resolve` disables it.
7. **`internal/skillgen`** — renders the `skills/` tree (an agent-facing command reference) from the live Cobra tree + embedded templates via the hidden `dscrd generate-skills` command. Skills are generated artifacts; CI fails on drift.

**Data flow (typical read):** flags → `buildClient` (token from auth store, base URL overridable via `DSCRD_API_BASE`) → `guildID`/`channelID` resolution (snowflake pass-through, else case-insensitive name match over the live guild/channel list) → `client.Do` → unmarshal into a compact item struct → `output.Emit`.

**Write-verb contract:** every mutating command honors `--dry-run` (prints the would-be call, sends nothing — and needs no credentials when the target is a snowflake ID, via `clientAndChannel`/`clientAndGuild`) and `--raw` (prints the raw Discord JSON response).

External dependencies (deliberately minimal): `spf13/cobra` (command tree), `BurntSushi/toml` (config), `zalando/go-keyring` (OS keyring), `golang.org/x/term` (hidden token input). No Discord SDK — the REST surface is hand-written, which keeps the binary small and gives immediate access to new endpoints (e.g. guild message search).

## Layout

```
cmd/dscrd/main.go        # entry point; error → exit-code mapping
internal/
├── api/                 # REST client, error/hint mapping, multipart, retries
├── auth/                # encrypted profile store, keyring, token resolution
├── commands/            # command tree: one file per group + paired _test.go
│   ├── root.go          #   group registration, emit() helper
│   ├── context.go       #   GlobalFlags + persistent flag binding
│   ├── clientutil.go    #   buildClient, guildID/channelID, dry-run helpers
│   ├── testhelper_test.go     # runCmd/markCovered + coverage registry
│   ├── zz_coverage_test.go    # meta-test: every leaf must be exercised
│   └── branding_test.go       # bans foreign-platform tokens in repo text
├── output/              # concise/json/jsonl/table emitters
├── resolve/             # kind:id → name disk cache; IsSnowflake
└── skillgen/            # skills/ generator + embedded templates
skills/                  # GENERATED agent skills (dscrd, dscrd-shared, dscrd-<group>)
npm/                     # npm wrapper (@howar31/dscrd): postinstall binary download + SHA256 verify
scripts/smoke.sh         # live end-to-end test against a real server (env-gated)
.github/workflows/       # ci.yml (fmt/vet/test/goreleaser check + skills drift), release.yml
.goreleaser.yaml         # darwin/linux × amd64/arm64 archives + checksums
VERSION / version.go     # version SSOT, embedded via go:embed
PLAN.md                  # design spec (zh-TW, user-facing)
```

## Conventions

- Code comments and test names in English; SPEC.md/CLAUDE.md in English; PLAN.md in zh-TW.
- One file per command group; compact item structs implement `Concise()`; tests exercise commands end-to-end against `httptest` servers through the `DSCRD_API_BASE` override.
- Test fixtures use scrubbed identifiers only (`Alice`/`Bob`, patterned snowflakes like `111111111111111111`); never real names, IDs, or tokens.
- Command annotations are the metadata SSOT: `discordEndpoint` (API route), `write` ("true" for mutations), `perms` (Discord permission names), `intents` (privileged intents). Skills render these; adding a command without them produces an incomplete skill.
- Tokens and webhook tokens are never printed or logged; `auth status` reports token state (`set`/`locked`/`empty`), never values.
- Untrusted input never shapes a request or the terminal:
  - ID flags listed in `idFlags` (`input.go`) must be snowflakes and are validated in the root `PersistentPreRunE`; a new flag that is spliced into a path must be added there (or validated in `RunE` for positional args). `api.Client` additionally refuses dot segments and control characters.
  - A path that embeds a credential goes through `Client.DoSecretPath`, which reports a redacted path in every error.
  - Posting verbs build their payload with `messageBody`, which suppresses mention pings (`allowed_mentions.parse = []`) unless `--allow-mentions` is passed.
  - Remote text is rendered through `output.Sanitize` (line breaks escaped, control and bidi characters dropped); `main` also wraps stdout/stderr in `output.SafeWriter`.
  - `DSCRD_API_BASE` must be https (http for loopback only); `file download` is https-only on the CDN hosts, re-checks redirects, and needs `--force` to overwrite.

## Verification

- `go test ./...` — full suite. Coverage of the command surface is *enforced*, not aspirational: `zz_coverage_test.go` walks the Cobra tree and fails if any runnable leaf command was not exercised via `runCmd`/`markCovered` during the package's test run (consequence: never include `TestZZ` in a `-run`-filtered invocation — the registry will be incomplete by construction).
- `branding_test.go` scans all repo text files (`.go .md .tmpl .yml .yaml .json .sh .js`) and fails on foreign-platform tokens, keeping the project self-contained.
- `generateskill_test.go` regenerates skills in-memory and diffs against the committed `skills/` tree (drift guard, mirrored by the CI `skill` job).
- `scripts/smoke.sh` — live smoke test (auth → guild → channel → msg send/react/edit → search → delete) against a real test server; requires `DSCRD_SMOKE_GUILD` / `DSCRD_SMOKE_CHANNEL` and working credentials. Run before releases.

## Deploy

- **CI** (`.github/workflows/ci.yml`): gofmt check, `go vet`, build, test, goreleaser config validation, skills drift guard.
- **Release** (`.github/workflows/release.yml`): driven by the committed `VERSION` file — pushing a `VERSION` change to `main` gates on whether tag `v<VERSION>` exists, then creates the tag, runs goreleaser (darwin/linux × amd64/arm64 tar.gz + SHA256 checksums + build provenance attestation), and publishes the npm wrapper `@howar31/dscrd` (skipped for prerelease versions containing `-`). The tag is an artifact of the release, not its trigger. Requires the `NPM_TOKEN` repository secret.
- **Install paths**: npm wrapper (postinstall downloads the platform binary from GitHub Releases and verifies its checksum), `go install github.com/howar31/dscrd/cmd/dscrd@latest`, or a release binary.

## Known Limitations / Non-goals

Platform-imposed (bot model):
- No user-token automation, ever (ToS; account-termination risk). The bot sees only servers it was invited to.
- Message search is per-guild only; no cross-guild or DM search. A cold index returns 202/`110000` (auto-retried); newly sent messages take minutes to appear in results.
- Bot DMs require a mutual server (`50007` otherwise); bots cannot enumerate their DM channels; no access to any user's personal DM inbox.
- Presence/online status is unavailable over REST (gateway-only), so a stateless CLI cannot expose it.
- Reading message content and listing members require privileged intents (MESSAGE_CONTENT, GUILD_MEMBERS) toggled in the Developer Portal; error hints point there.

Deliberate non-goals:
- No gateway/event listening — that is a bot framework's job, not a CLI's.
- No Windows binaries yet (npm wrapper supports darwin/linux; Windows users can `go install`).

Known behaviors:
- Channel/guild name resolution is case-insensitive first-match; scripts should prefer IDs (`#bot` vs a `#BOT` category both match).
- Embed-only messages render as `[embed: title — description-excerpt]`; attachments as `[file:name url]`.

## Key Decisions

- **Bot token only** — the only ToS-compliant automation model on this platform; the CLI's scope is framed around it honestly rather than emulating a user session.
- **Hand-written REST client over an SDK** — the used surface is small and stable; avoiding a framework dependency keeps startup instant (critical for one-shot agent calls) and gives day-one access to newly public endpoints such as guild message search.
- **Stateless one-shot process model** — no daemon; rate-limit handling is reactive (429 retry) which is sufficient at CLI call rates.
- **Coverage meta-test instead of discipline** — the command tree is walked mechanically; untested commands are a CI failure, so coverage cannot silently rot.
- **Skills as generated artifacts** — the agent-facing reference is rendered from the command tree itself and drift-guarded, so it can never lie about the CLI.
- **Encrypted-at-rest credentials with keyring-first key storage** — tokens are useless on disk without the OS keyring (or key file) even if the config file leaks.
- **`VERSION` file as release SSOT** — releases are reviewable PRs that change one file; tags are outputs, not inputs.
