# dscrd 機能マニュアル(日本語)

dscrd は、Bot アカウント経由で Discord サーバーを読み書きするコマンドラインツールです。AI エージェントとターミナル利用者の両方を想定しています。

このマニュアルは、機能ごとに「何ができるか」「どう使うか」「どのソースが担っているか」をまとめたものです。

- フラグの完全な一覧は `dscrd <グループ> <コマンド> --help` が正です。ここでは主要なものだけを載せています。
- 設計上の決定は [SPEC.md](../SPEC.md)、導入手順の原文は [README.md](../README.md) にあります。
- エージェント向けの英語リファレンスは、コマンド定義から自動生成される [skills/](../skills/) です。

## 目次

1. [全体像](#1-全体像)
2. [導入と認証](#2-導入と認証)
3. [共通の使い方](#3-共通の使い方)
4. [機能別リファレンス](#4-機能別リファレンス)
5. [安全装置](#5-安全装置)
6. [Discord 側の制約](#6-discord-側の制約)
7. [エージェント向けスキル](#7-エージェント向けスキル)
8. [開発者向け情報](#8-開発者向け情報)
9. [ソースコード対応表](#9-ソースコード対応表)

## 1. 全体像

dscrd は「1 コマンド = 1 回の REST 呼び出し」で動きます。常駐プロセスやゲートウェイ接続はありません。Bot が「オンライン」である必要もありません。

読み取りコマンドの典型的な処理の流れは次のとおりです。

| 順 | 処理 | 担当するソース |
|---|---|---|
| 1 | コマンドとフラグを解釈する | [root.go](../internal/commands/root.go) の `NewRootCommand`、[context.go](../internal/commands/context.go) の `bindGlobalFlags` |
| 2 | ID フラグが Discord の ID 形式か検証する | [input.go](../internal/commands/input.go) の `validateIDFlags` |
| 3 | トークンを解決して API クライアントを作る | [clientutil.go](../internal/commands/clientutil.go) の `buildClient`、[token.go](../internal/auth/token.go) の `ResolveToken` |
| 4 | サーバー名・チャンネル名を ID に解決する | [clientutil.go](../internal/commands/clientutil.go) の `guildID` / `channelID` |
| 5 | Discord の API を呼ぶ | [client.go](../internal/api/client.go) の `Do` |
| 6 | 応答を表示用の小さな構造体に詰め替える | 各コマンドファイルの `Concise()` を持つ型 |
| 7 | 指定の形式で出力する | [output.go](../internal/output/output.go) の `Emit` |

エラーは [main.go](../cmd/dscrd/main.go) で終了コードに変換されます。

## 2. 導入と認証

### インストール

```bash
npm install -g @howar31/dscrd          # 対応バイナリを取得し SHA256 を検証
brew install howar31/tap/dscrd         # macOS
go install github.com/howar31/dscrd/cmd/dscrd@latest
```

npm 版は、インストール時に [install.js](../npm/install.js) がバイナリをダウンロードします。実行時は [run.js](../npm/run.js) がそのバイナリを起動します。

### Bot の準備

1. <https://discord.com/developers/applications> でアプリケーションを作ります。
2. **Bot** 画面でトークンを控え、必要な Privileged Gateway Intents を有効にします。
   - **MESSAGE CONTENT INTENT**: メッセージ本文の読み取りと検索に必要です。
   - **SERVER MEMBERS INTENT**: `member list` に必要です。
3. トークンを保存します。`--token` を省くと、画面に表示されない入力欄で尋ねられます。

   ```bash
   dscrd auth set-token --name <ラベル> --application-id <アプリケーションID> --default-guild <サーバーID>
   ```

4. 招待 URL を作り、ブラウザで開いてサーバーに追加します。

   ```bash
   dscrd auth invite-url                    # 読み書き向けの権限セット
   dscrd auth invite-url --permissions read # 読み取りのみ
   ```

5. 動作を確認します。

   ```bash
   dscrd auth test
   dscrd guild list
   ```

### auth コマンド

ソース: [auth.go](../internal/commands/auth.go)

| コマンド | 内容 |
|---|---|
| `auth set-token` | トークンをプロファイルとして保存します。既存プロファイルの上書きには `--force` が必要です |
| `auth status` | プロファイル一覧と、有効なトークンの確認結果を表示します。トークンの値は出しません |
| `auth test` | 有効なトークンで Discord に問い合わせ、Bot 名を表示します |
| `auth switch <名前>` | 既定で使うプロファイルを切り替えます |
| `auth rename <旧> <新>` | プロファイル名を変更します |
| `auth logout` | プロファイルを削除します(`--name` 省略時は現在のもの) |
| `auth invite-url` | Bot をサーバーに招待する URL を作ります。`--permissions` は `read` / `write` / `admin` か、権限ビットの数値です |

複数の Bot を使う場合は、`--name` でプロファイルを分けます。一度だけ別の Bot を使うなら `dscrd --profile <名前> ...` と指定します。

### トークンの保存方式

| 項目 | 内容 | 担当するソース |
|---|---|---|
| 保存先 | `~/.config/dscrd/config.toml`(権限 0600) | [store.go](../internal/auth/store.go) の `Load` / `Save` |
| 暗号化 | AES-256-GCM。値は `enc:v1:` で始まります | [crypto.go](../internal/auth/crypto.go) の `encryptValue` / `decryptValue` |
| 鍵の置き場所 | OS のキーリング。使えない環境では設定ファイルと同じ場所の `.encryption_key`(0600) | [keyprovider.go](../internal/auth/keyprovider.go) の `resolveBackend` / `loadKey` |
| 解決の優先順位 | `DSCRD_TOKEN` → `--profile` → `DSCRD_PROFILE` → 設定の `active` | [token.go](../internal/auth/token.go) の `ResolveToken` |

鍵の置き場所は `DSCRD_KEYRING_BACKEND`(`auto` / `keyring` / `file`)で指定できます。ファイル方式では鍵と暗号文が同じディレクトリに並ぶため、保護の強さはファイル権限に依存します。

## 3. 共通の使い方

### グローバルフラグ

定義: [context.go](../internal/commands/context.go)

| フラグ | 意味 |
|---|---|
| `--format concise\|json\|jsonl\|table` | 出力形式。既定は `concise` |
| `--guild <ID\|名前>` | 対象サーバー。省略時はプロファイルの `default_guild` |
| `--profile <名前>` | 使うプロファイル |
| `--raw` | Discord の応答 JSON をそのまま表示します |
| `--dry-run` | 送信せず、呼び出す予定の API を表示します |
| `--no-resolve` | ID を名前に置き換える処理を止めます |

### サーバーとチャンネルの指定

- `--guild` と `--channel` は、ID と名前のどちらでも指定できます。チャンネル名は `#general` のように `#` を付けても構いません。
- 名前は大文字小文字を区別せず、最初に一致したものが選ばれます。スクリプトでは ID を使うほうが確実です。
- `--message`、`--user`、`--role`、`--thread`、`--id` などは ID(17〜20 桁の数字)だけを受け付けます。

### 出力形式

実装: [output.go](../internal/output/output.go)、[render.go](../internal/output/render.go)

| 形式 | 内容 |
|---|---|
| `concise` | 1 項目 1 行。エージェントが読むときのトークン消費を抑えます |
| `json` | 整形した JSON 配列 |
| `jsonl` | 1 項目 1 行の JSON |
| `table` | 列を揃えた表 |

メッセージは次の形で表示されます。

```console
$ dscrd msg read --channel "#general" --limit 2
Alice: anyone up for a review? [07-03 14:02] (333333333333333333)
Bob: on it [07-03 14:05] (333333333333333334)
```

- 添付ファイルは `[file:名前 URL]`、埋め込みは `[embed: タイトル — 説明の抜粋]` として本文に続きます。
- `<@ID>` や `<#ID>` は、名前に置き換えて表示されます([msg.go](../internal/commands/msg.go) の `prettifyMentions`、[resolve.go](../internal/resolve/resolve.go))。結果は `~/.config/dscrd/cache/resolve.json` に保存されます。
- `concise` と `table` では、本文中の改行は `\n` という文字で表示され、制御文字は取り除かれます([sanitize.go](../internal/output/sanitize.go) の `Sanitize`)。

### 本文の渡し方

実装: [input.go](../internal/commands/input.go) の `readContent`

- 短い文は `--text "..."` で渡します。
- 複数行の文は `--text-file <パス>` で渡します。`-` を指定すると標準入力から読みます。
- シェルの `"\n"` は改行になりません。改行を含めるときは `--text-file` を使ってください。

### 終了コード

| コード | 意味 |
|---|---|
| `0` | 成功 |
| `3` | 認証の問題 |
| `4` | 対象が見つからない |
| `5` | レート制限 |
| `1` | その他 |

対応付けは [errors.go](../internal/api/errors.go) の `ExitCode` にあります。権限や Intent の不足が原因と考えられるエラーには、対処を示す `hint:` 行が付きます。

### 環境変数

| 変数 | 用途 |
|---|---|
| `DSCRD_TOKEN` | 保存済みプロファイルより優先して使うトークン |
| `DSCRD_PROFILE` | 使うプロファイル(`--profile` が優先) |
| `DSCRD_CONFIG` | 設定ファイルのパス |
| `DSCRD_KEYRING_BACKEND` | 鍵の置き場所(`auto` / `keyring` / `file`) |
| `DSCRD_API_BASE` | API のベース URL。https 必須で、http はループバックのみ許可されます |
| `DSCRD_CDN_ALLOW_HOST` | `file download` で追加で許可するホスト(テスト用) |
| `DSCRD_RELEASE_API` | `version --check` の問い合わせ先(テスト用) |

## 4. 機能別リファレンス

表の「書込」に ✓ があるコマンドは、`--dry-run` で事前確認できます。「権限 / Intent」は Bot に必要な Discord 側の設定です。

### メッセージ(msg)

ソース: [msg.go](../internal/commands/msg.go)

| コマンド | 内容 | 書込 | 権限 / Intent |
|---|---|---|---|
| `msg read` | チャンネルの最近のメッセージを古い順に表示します。`--before` / `--after` / `--around` で位置を指定できます | | VIEW_CHANNEL, READ_MESSAGE_HISTORY / MESSAGE_CONTENT |
| `msg send` | メッセージを送ります。`--reply-to` で返信、`--file` でファイル添付 | ✓ | SEND_MESSAGES |
| `msg edit` | Bot が送ったメッセージを書き換えます | ✓ | |
| `msg delete` | メッセージを削除します | ✓ | MANAGE_MESSAGES |
| `msg react` / `msg unreact` | リアクションを付けます / Bot のリアクションを外します | ✓ | ADD_REACTIONS(`react` のみ) |
| `msg reactions` | ある絵文字でリアクションしたユーザーを一覧します | | |
| `msg pin` / `msg unpin` | ピン留めします / 外します | ✓ | MANAGE_MESSAGES |
| `msg pins` | ピン留めされたメッセージを一覧します | | MESSAGE_CONTENT |
| `msg permalink` | メッセージの Web URL を表示します | | |

```bash
dscrd msg send --channel "#general" --text "deploy finished" --reply-to 333333333333333334
dscrd msg react --channel "#general" --message 333333333333333334 --emoji "👀"
```

- 絵文字は Unicode か、カスタム絵文字の `名前:ID` で指定します(`reactionPath`)。
- ファイル添付は [multipart.go](../internal/api/multipart.go) の `DoMultipart` が担います。
- 送信するメッセージのメンションは、既定では通知されません。通知したいときは `--allow-mentions` を付けます。

### スレッドとフォーラム(thread)

ソース: [thread.go](../internal/commands/thread.go)

| コマンド | 内容 | 書込 | 権限 / Intent |
|---|---|---|---|
| `thread list` | サーバーのアクティブなスレッドを一覧します。`--archived --channel` でアーカイブ済みを表示 | | |
| `thread create` | スレッドを作ります。`--from-message` でメッセージから、`--text` でフォーラム投稿として作成 | ✓ | CREATE_PUBLIC_THREADS |
| `thread read` | スレッド内のメッセージを表示します | | MESSAGE_CONTENT |
| `thread reply` | スレッドに返信します | ✓ | SEND_MESSAGES_IN_THREADS |
| `thread archive` | スレッドをアーカイブします | ✓ | MANAGE_THREADS |

フォーラムチャンネルへの投稿は `thread create --channel <フォーラム> --name <題名> --text <本文>` です。

### 検索(search)

ソース: [search.go](../internal/commands/search.go)

| コマンド | 内容 | 権限 / Intent |
|---|---|---|
| `search messages` | サーバー内のメッセージを全文検索します | READ_MESSAGE_HISTORY / MESSAGE_CONTENT |

`--content`(検索語)、`--author`、`--channel`、`--has`(`link` / `file` / `image` など)、`--pinned`、`--sort-by`、`--sort-order`、`--limit`、`--offset` で絞り込みます。結果の最後に総件数が表示されます。

検索インデックスが準備中のときは Discord が待機を求めるため、クライアントが自動で再試行します([client.go](../internal/api/client.go) の `retryDelay`)。

### サーバーとチャンネル(guild / channel)

ソース: [guild.go](../internal/commands/guild.go)、[channel.go](../internal/commands/channel.go)

| コマンド | 内容 | 書込 | 権限 |
|---|---|---|---|
| `guild list` | Bot が参加しているサーバーを一覧します | | |
| `guild info` | サーバーの詳細(人数、所有者、説明)を表示します | | |
| `channel list` | サーバーのチャンネルを一覧します | | |
| `channel info` | チャンネルの詳細を表示します | | |
| `channel create` | チャンネルを作ります。`--type` は `text` / `voice` / `category` / `announcement` / `stage` / `forum` / `media` | ✓ | MANAGE_CHANNELS |
| `channel edit` | 名前やトピックを変更します | ✓ | MANAGE_CHANNELS |
| `channel topic` | トピックだけを設定します | ✓ | MANAGE_CHANNELS |
| `channel delete` | チャンネルを削除します | ✓ | MANAGE_CHANNELS |

### メンバーとロール(member / role / user)

ソース: [member.go](../internal/commands/member.go)、[role.go](../internal/commands/role.go)、[user.go](../internal/commands/user.go)

| コマンド | 内容 | 書込 | 権限 / Intent |
|---|---|---|---|
| `member list` | メンバーを一覧します。`--after` で続きを取得 | | GUILD_MEMBERS |
| `member info` | メンバー 1 人の詳細を表示します | | GUILD_MEMBERS |
| `member search` | 名前の前方一致でメンバーを探します | | |
| `role list` | ロールを一覧します | | |
| `role create` / `role edit` / `role delete` | ロールを作成 / 変更 / 削除します | ✓ | MANAGE_ROLES |
| `role assign` / `role unassign` | メンバーにロールを付けます / 外します | ✓ | MANAGE_ROLES |
| `user info` | ユーザーのプロフィールを表示します | | |
| `user me` | Bot 自身のユーザー情報を表示します | | |

### ダイレクトメッセージ(dm)

ソース: [dm.go](../internal/commands/dm.go)

| コマンド | 内容 | 書込 |
|---|---|---|
| `dm send` | Bot としてユーザーに DM を送ります | ✓ |
| `dm read` | Bot とそのユーザーの DM 履歴を表示します | |

どちらも、まず DM チャンネルを開いてから(`openDM`)メッセージを扱います。相手と Bot が同じサーバーにいる必要があります。

### 添付ファイル(file)

ソース: [file.go](../internal/commands/file.go)

| コマンド | 内容 |
|---|---|
| `file download <URL>` | Discord の CDN から添付ファイルを保存します。保存先は `--out`、省略時は URL のファイル名 |

- 取得元は `cdn.discordapp.com` と `media.discordapp.net` の https に限られます(`checkCDNURL`)。
- 同名のファイルがあると失敗します。上書きするときは `--force` を付けます。
- 添付ファイルの URL は `msg read` の `[file:名前 URL]` から得られます。

### 招待と Webhook(invite / webhook)

ソース: [invite.go](../internal/commands/invite.go)、[webhook.go](../internal/commands/webhook.go)

| コマンド | 内容 | 書込 | 権限 |
|---|---|---|---|
| `invite list` | サーバーの招待を一覧します | | MANAGE_GUILD |
| `invite create` | チャンネルの招待を作ります。`--max-age`(秒)、`--max-uses` | ✓ | CREATE_INSTANT_INVITE |
| `invite info <コード>` | 招待の詳細を表示します | | |
| `invite delete <コード>` | 招待を取り消します | ✓ | MANAGE_CHANNELS |
| `webhook list` | サーバーまたはチャンネルの Webhook を一覧します | | MANAGE_WEBHOOKS |
| `webhook create` | Webhook を作ります。トークンはこのとき一度だけ表示されます | ✓ | MANAGE_WEBHOOKS |
| `webhook delete <ID>` | Webhook を削除します | ✓ | MANAGE_WEBHOOKS |
| `webhook execute` | Webhook 経由でメッセージを投稿します。`--id` と `--token` が必要 | ✓ | |

- 招待は URL ではなくコードだけを渡します。
- `webhook execute` は Webhook のトークンだけで動き、Bot のトークンは使いません。Webhook のトークンは、エラーや `--dry-run` の表示では `<token>` に置き換えられます。

### イベント・投票・監査ログ・自動モデレーション

ソース: [event.go](../internal/commands/event.go)、[poll.go](../internal/commands/poll.go)、[auditlog.go](../internal/commands/auditlog.go)、[automod.go](../internal/commands/automod.go)

| コマンド | 内容 | 書込 | 権限 |
|---|---|---|---|
| `event list` | 予定されたイベントを一覧します | | |
| `event create` | イベントを作ります。`--location` で外部開催、`--channel` でボイスチャンネル開催 | ✓ | MANAGE_EVENTS |
| `event edit` / `event delete` | イベントを変更 / 削除します | ✓ | MANAGE_EVENTS |
| `poll create` | 投票を投稿します。`--answer` を 2〜10 回指定、`--duration` は時間単位 | ✓ | SEND_MESSAGES |
| `poll results` | ある選択肢に投票したユーザーを一覧します。`--answer` は 1 始まりの位置 | | |
| `poll end` | 投票をすぐに締め切ります | ✓ | |
| `audit-log read` | 監査ログを表示します。`--user` と `--action` で絞り込み | | VIEW_AUDIT_LOG |
| `automod list` / `automod info` | 自動モデレーションのルールを一覧 / 表示します | | MANAGE_GUILD |
| `automod create` | キーワードをブロックするルールを作ります。`--keyword` は複数指定可 | ✓ | MANAGE_GUILD |
| `automod delete` | ルールを削除します | ✓ | MANAGE_GUILD |

イベントの時刻は ISO 8601 形式(例: `2026-08-01T19:00:00+09:00`)で指定します。

### 絵文字とスタンプ(emoji / sticker)

ソース: [emoji.go](../internal/commands/emoji.go)、[sticker.go](../internal/commands/sticker.go)

| コマンド | 内容 |
|---|---|
| `emoji list` / `emoji info` | サーバーのカスタム絵文字を一覧 / 表示します |
| `sticker list` / `sticker info` | サーバーのスタンプを一覧 / 表示します |

### 任意の API 呼び出し(api)

ソース: [api.go](../internal/commands/api.go)

専用コマンドがない操作は、REST のエンドポイントを直接呼べます。出力は常に応答の JSON です。

```bash
dscrd api GET /users/@me
dscrd api POST /channels/222222222222222222/messages --body '{"content":"hi"}'
dscrd api GET /guilds/111111111111111111/members --query limit=10
```

- メソッドは `GET` / `POST` / `PATCH` / `PUT` / `DELETE` です。
- 本文は `--body`(JSON 文字列)か `--body-file`(ファイル)で渡します。
- このコマンドは Bot の権限が及ぶすべての操作に届きます。実行前に `--dry-run` で内容を確かめてください。

### バージョン(version)

ソース: [version.go](../internal/commands/version.go)、[version.go(ルート)](../version.go)

`dscrd version` で現在のバージョンを表示します。`--check` を付けると、最新のリリースと比べます。

## 5. 安全装置

| 仕組み | 内容 | 担当するソース |
|---|---|---|
| 事前確認 | 書き込みコマンドは `--dry-run` で、送る予定の API と本文を表示します。多くのコマンドでは、対象を ID で指定していれば認証情報も不要です | [clientutil.go](../internal/commands/clientutil.go) の `dryRunPrint` / `clientAndChannel` / `clientAndGuild` |
| ID の検証 | ID フラグに ID 以外が渡されると、API を呼ぶ前に拒否します | [input.go](../internal/commands/input.go) の `validateIDFlags` |
| パスの検証 | `..` や制御文字を含むリクエストパスを拒否します | [client.go](../internal/api/client.go) の `checkPath` |
| メンションの抑止 | 投稿するテキストに `@everyone` などが含まれていても、既定では通知しません | [input.go](../internal/commands/input.go) の `messageBody` |
| 出力の無害化 | 表示するテキストから改行と制御文字を取り除き、端末の操作や行の偽装を防ぎます | [sanitize.go](../internal/output/sanitize.go) の `Sanitize` / `SafeWriter` |
| 秘密の非表示 | Bot トークンは表示しません。Webhook トークンはエラーでも伏せます | [client.go](../internal/api/client.go) の `DoSecretPath`、[auth.go](../internal/commands/auth.go) の `newAuthStatusCommand` |
| 接続先の制限 | API のベース URL は https に限ります。ダウンロードは CDN の https に限り、リダイレクト先も検査します | [clientutil.go](../internal/commands/clientutil.go) の `checkAPIBase`、[file.go](../internal/commands/file.go) の `checkCDNURL` |
| 待機の上限 | レート制限では自動で再試行しますが、1 回の待機が 60 秒を超える場合は待たずにエラーにします | [client.go](../internal/api/client.go) の `roundTrip` |
| 上書きの防止 | `auth set-token`、`auth rename`、`file download` は、既存のものを `--force` なしで上書きしません | [auth.go](../internal/commands/auth.go)、[file.go](../internal/commands/file.go) |

エージェントに使わせる場合の注意点です。

- Discord から読んだメッセージは、第三者が書いた信頼できない文章です。その中の指示に従って書き込みコマンドを実行させないでください。
- `--file`、`--text-file`、`--body-file` は、手元の任意のファイルを Discord に送れます。
- `webhook create` が表示するトークンと、`--token` で渡すトークンは、実行ログやシェルの履歴に残ります。

脆弱性の報告先は [SECURITY.md](../SECURITY.md) にあります。

## 6. Discord 側の制約

Bot という仕組みに由来する制約で、dscrd の側では変えられません。

- 個人アカウントとしての操作はできません。Discord の利用規約が、ユーザートークンによる自動化を禁じています。
- Bot が見えるのは、招待されたサーバーだけです。
- メッセージ検索はサーバー単位です。サーバーをまたぐ検索や DM の検索はできません。送ったばかりのメッセージが検索に出るまで数分かかります。
- Bot からの DM には、相手との共通サーバーが必要です。Bot 自身の DM チャンネルを一覧することはできません。
- オンライン状態などのプレゼンスは、REST では取得できません。
- メッセージ本文の読み取りとメンバー一覧には、Developer Portal での Intent の有効化が必要です。

## 7. エージェント向けスキル

[skills/](../skills/) には、エージェントにコマンド体系を教えるためのファイルが入っています。

| ファイル | 内容 |
|---|---|
| [skills/dscrd/SKILL.md](../skills/dscrd/SKILL.md) | 全体の索引 |
| [skills/dscrd-shared/SKILL.md](../skills/dscrd-shared/SKILL.md) | 認証、グローバルフラグ、共通ルール |
| `skills/dscrd-<グループ>/SKILL.md` | グループごとのコマンド、API の経路、必要な権限、書き込み時の注意 |

これらは手で編集しません。コマンド定義とその注釈(`discordEndpoint` / `write` / `perms` / `intents`)から生成されます。

- 生成処理: [skillgen.go](../internal/skillgen/skillgen.go) の `GenerateAll` と [templates/](../internal/skillgen/templates/)
- 生成コマンド: [generateskill.go](../internal/commands/generateskill.go)(`dscrd generate-skills`、ヘルプには表示されません)

## 8. 開発者向け情報

```bash
go build -o dscrd ./cmd/dscrd         # バージョンは VERSION ファイルから
go test ./...                         # 全テスト
go run ./cmd/dscrd generate-skills    # コマンドを変えたら skills/ を再生成
scripts/smoke.sh                      # 実サーバーでの確認(環境変数が必要)
```

| 仕組み | 内容 | 担当するソース |
|---|---|---|
| コマンドの網羅テスト | すべての末端コマンドがテストで実行されていないと失敗します | [zz_coverage_test.go](../internal/commands/zz_coverage_test.go)、[testhelper_test.go](../internal/commands/testhelper_test.go) |
| スキルのずれ検出 | コミットされた `skills/` が最新の生成結果と一致しないと失敗します | [generateskill_test.go](../internal/commands/generateskill_test.go) |
| 実サーバー確認 | 認証から送信、検索、削除までを通しで実行します | [smoke.sh](../scripts/smoke.sh) |
| CI | 整形、vet、ビルド、テスト、スキルのずれ検出 | [ci.yml](../.github/workflows/ci.yml) |
| リリース | `VERSION` の変更が `main` に入ると、タグ作成、バイナリのビルド、npm 公開が走ります | [release.yml](../.github/workflows/release.yml)、[.goreleaser.yaml](../.goreleaser.yaml) |

コマンドを追加するときは、`internal/commands/` にグループごとのファイルと対になるテストを置き、[root.go](../internal/commands/root.go) の `NewRootCommand` に登録します。規約の全体は [SPEC.md](../SPEC.md) と [CLAUDE.md](../CLAUDE.md) にあります。

## 9. ソースコード対応表

| 役割 | ソース |
|---|---|
| 起動と終了コード | [cmd/dscrd/main.go](../cmd/dscrd/main.go) |
| コマンドの登録 | [internal/commands/root.go](../internal/commands/root.go) |
| グローバルフラグ | [internal/commands/context.go](../internal/commands/context.go) |
| クライアント生成、名前解決、dry-run | [internal/commands/clientutil.go](../internal/commands/clientutil.go) |
| 入力の検証、本文の読み込み、メンション抑止 | [internal/commands/input.go](../internal/commands/input.go) |
| 各機能のコマンド | [internal/commands/](../internal/commands/)(グループごとに 1 ファイル) |
| REST クライアント、再試行 | [internal/api/client.go](../internal/api/client.go) |
| ファイル送信 | [internal/api/multipart.go](../internal/api/multipart.go) |
| エラーとヒント、終了コード | [internal/api/errors.go](../internal/api/errors.go) |
| プロファイルの保存 | [internal/auth/store.go](../internal/auth/store.go) |
| トークンの暗号化 | [internal/auth/crypto.go](../internal/auth/crypto.go) |
| 鍵の管理 | [internal/auth/keyprovider.go](../internal/auth/keyprovider.go) |
| トークンの解決 | [internal/auth/token.go](../internal/auth/token.go) |
| 出力形式 | [internal/output/output.go](../internal/output/output.go)、[internal/output/render.go](../internal/output/render.go) |
| 出力の無害化 | [internal/output/sanitize.go](../internal/output/sanitize.go) |
| ID から名前への変換とキャッシュ | [internal/resolve/resolve.go](../internal/resolve/resolve.go) |
| スキルの生成 | [internal/skillgen/skillgen.go](../internal/skillgen/skillgen.go) |
| npm 配布 | [npm/install.js](../npm/install.js)、[npm/run.js](../npm/run.js)、[npm/platform.js](../npm/platform.js) |
