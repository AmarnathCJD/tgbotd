# tgbotd

> A from-scratch **Telegram Bot API server** written in pure Go — drop-in for
> `telegram-bot-api`, backed directly by MTProto via
> [gogram](https://github.com/amarnathcjd/gogram).

Point any Bot API client at `http://127.0.0.1:8081/bot<TOKEN>/<method>` and
it just works. No `libtdjson`, no CGO, single static binary. Faster than
`api.telegram.org` on every cache-friendly path and matches the DC round-trip
on the rest.

Bot API surface up to **10.2** — 182 methods wired (9 as documented 501
stubs, see below), incl. Payments/Stars, Business accounts, Gifts, Stories,
Stickers (full CRUD), Games, Rich messages, Suggested posts, and 2 extension
methods (`resolveUsername`, `getMessages`).

---

## Quick start

```bash
git clone https://github.com/amarnathcjd/tgbotd
cd tgbotd
go run .
```

```bash
curl -X POST "http://127.0.0.1:8081/bot$TELEGRAM_BOT_TOKEN/getMe"
```

That's it. The server ships with public MTProto `api_id`/`api_hash` defaults
so the very first call authenticates the bot, caches its session under
`./data/`, and every subsequent call reuses it.

---

## Benchmark vs `api.telegram.org`

Serial p50 latency, n=20 with 5-request warm-up:

| method                          | tgbotd      | upstream | speedup   |
| ------------------------------- | ----------- | -------- | --------- |
| `getMe`                         | **0.55 ms** | 150 ms   | **~275×** |
| `getMyCommands` (cached)        | **0.5 ms**  | 152 ms   | **~300×** |
| `getWebhookInfo`                | **0.54 ms** | 153 ms   | **~285×** |
| `getUpdates` (short-poll, idle) | **0 ms**    | 3140 ms  | **∞**     |
| `resolveUsername`               | 193 ms      | —        | —         |
| `sendMessage`                   | 210 ms      | 240 ms   | ≈1.1×     |

Cache-friendly reads stay sub-millisecond; network-bound writes hit the
physical DC RTT floor. Under 20-concurrent load tgbotd holds 1–4 ms p50 with
zero errors while upstream starts throwing 429s.

---

## Configuration

Environment only — no config files.

| Variable            | Default          | Purpose                                          |
| ------------------- | ---------------- | ------------------------------------------------ |
| `TGBOTD_HTTP_ADDR`  | `127.0.0.1:8081` | HTTP listen address                              |
| `TGBOTD_DATA_DIR`   | `./data`         | Session, cache, SQLite state                     |
| `TGBOTD_API_ID`     | `2040`           | MTProto app id                                   |
| `TGBOTD_API_HASH`   | (baked default)  | MTProto app hash                                 |
| `TGBOTD_TEST_DC`    | `false`          | Route to Telegram test DCs                       |
| `TGBOTD_LOG_LEVEL`  | `info`           | `debug` / `info` / `warn` / `error`              |

`.env.example` is committed as a template.

---

## Endpoints

| Path                             | Method  | Description                                        |
| -------------------------------- | ------- | -------------------------------------------------- |
| `/bot<token>/<method>`           | POST    | Full Bot API method dispatch                       |
| `/file/bot<token>/<path>`        | GET     | Stream a file (HMAC-verified, per-bot signed path) |
| `/stats`                         | GET     | Per-method call counts, p50/p99 latency, active bots |

---

## Method coverage (182)

<details>
<summary><strong>Getting updates</strong></summary>

`getUpdates`, `setWebhook`, `deleteWebhook`, `getWebhookInfo`

Webhook delivery runs as a per-bot goroutine with exponential backoff (1 s →
300 s), `X-Telegram-Bot-Api-Secret-Token` support, `drop_pending_updates`,
and a per-update attempt cap so one broken endpoint can't wedge the pipeline.

</details>

<details>
<summary><strong>Available methods — send / message ops</strong></summary>

`getMe`, `logOut`, `close`, `sendMessage`, `forwardMessage`, `forwardMessages`,
`copyMessage`, `copyMessages`, `sendPhoto`, `sendAudio`, `sendDocument`,
`sendVideo`, `sendAnimation`, `sendVoice`, `sendVideoNote`, `sendMediaGroup`,
`sendSticker`, `sendLocation`, `sendVenue`, `sendContact`, `sendPoll`,
`sendDice`, `sendChatAction`, `setMessageReaction`, `getUserProfilePhotos`

</details>

<details>
<summary><strong>Updating messages</strong></summary>

`editMessageText`, `editMessageCaption`, `editMessageMedia`,
`editMessageReplyMarkup`, `editMessageLiveLocation`, `stopMessageLiveLocation`,
`stopPoll`, `deleteMessage`, `deleteMessages`

</details>

<details>
<summary><strong>Files</strong></summary>

`getFile`, `uploadStickerFile`, `GET /file/bot<token>/<path>`.

2 GB per file (4 GB premium) via gogram's parallel chunk workers.
`file_id` uses the tdlib v4 wire format (base64url-nopad + RLE zero-run
compression) so ids interoperate with every Bot API client library.
`file_path` is an HMAC-SHA256-signed opaque blob keyed by the bot token —
file ids can't be replayed against a different bot.

URLs passed to `sendPhoto`/`sendDocument`/etc. are downloaded server-side and
re-uploaded via MTProto, so Telegram's OG-tag crawler is bypassed and any
HTTPS host works.

</details>

<details>
<summary><strong>Chat management</strong></summary>

`getChat`, `getChatMember`, `getChatMemberCount`, `getChatAdministrators`,
`banChatMember`, `unbanChatMember`, `restrictChatMember`,
`promoteChatMember`, `leaveChat`, `setChatTitle`, `setChatDescription`,
`setChatPhoto`, `deleteChatPhoto`, `setChatPermissions`,
`setChatAdministratorCustomTitle`, `pinChatMessage`, `unpinChatMessage`,
`unpinAllChatMessages`, `getUserChatBoosts`

</details>

<details>
<summary><strong>Invite links &amp; join requests</strong></summary>

`exportChatInviteLink`, `createChatInviteLink`, `editChatInviteLink`,
`revokeChatInviteLink`, `approveChatJoinRequest`, `declineChatJoinRequest`

</details>

<details>
<summary><strong>Forum topics</strong></summary>

`createForumTopic`, `editForumTopic`, `closeForumTopic`, `reopenForumTopic`,
`deleteForumTopic`, `unpinAllForumTopicMessages`, `closeGeneralForumTopic`,
`reopenGeneralForumTopic`, `hideGeneralForumTopic`, `unhideGeneralForumTopic`

</details>

<details>
<summary><strong>Bot config</strong></summary>

`setMyCommands`, `getMyCommands`, `deleteMyCommands`, `setMyName`, `getMyName`,
`setMyDescription`, `getMyDescription`, `setMyShortDescription`,
`getMyShortDescription`, `setMyDefaultAdministratorRights`,
`getMyDefaultAdministratorRights`, `setChatMenuButton`, `getChatMenuButton`

Reads are backed by a byte cache and a shared BotInfo cache so
`getMyName` + `getMyDescription` + `getMyShortDescription` share **one**
MTProto round-trip (three RPCs → one on cold, zero on warm).

</details>

<details>
<summary><strong>Payments &amp; Stars</strong></summary>

`sendInvoice`, `createInvoiceLink`, `answerPreCheckoutQuery`,
`answerShippingQuery`, `getStarTransactions`, `refundStarPayment`,
`getMyStarBalance`, `editUserStarSubscription`

</details>

<details>
<summary><strong>Stickers (full set)</strong></summary>

`createNewStickerSet`, `addStickerToSet`, `deleteStickerSet`,
`setStickerSetTitle`, `setStickerSetThumbnail`,
`setCustomEmojiStickerSetThumbnail`, `deleteStickerFromSet`,
`replaceStickerInSet`, `setStickerPositionInSet`, `setStickerEmojiList`,
`setStickerKeywords`, `setStickerMaskPosition`, `getStickerSet`,
`getCustomEmojiStickers`, `getForumTopicIconStickers`, `getAvailableGifts`

</details>

<details>
<summary><strong>Games</strong></summary>

`sendGame`, `setGameScore`, `getGameHighScores`

</details>

<details>
<summary><strong>Business account APIs</strong></summary>

`getBusinessConnection`, `getBusinessAccountGifts`,
`getBusinessAccountStarBalance`, `readBusinessMessage`,
`deleteBusinessMessages`, `setBusinessAccountName`,
`setBusinessAccountUsername`, `setBusinessAccountBio`,
`setBusinessAccountProfilePhoto`, `setBusinessAccountGiftSettings`,
`removeBusinessAccountProfilePhoto`, `transferBusinessAccountStars`,
`transferGift`

</details>

<details>
<summary><strong>Gifts</strong></summary>

`sendGift`, `giftPremiumSubscription`, `convertGiftToStars`, `upgradeGift`,
`getUserGifts`, `verifyUser`, `verifyChat`, `removeUserVerification`,
`removeChatVerification`

</details>

<details>
<summary><strong>Stories</strong></summary>

`postStory`, `editStory`, `deleteStory`

</details>

<details>
<summary><strong>Rich messages, ephemeral, suggested posts</strong></summary>

`sendRichMessage`, `editRichMessage`, `sendEphemeralMessage`,
`deleteEphemeralMessage`, `approveSuggestedPost`, `declineSuggestedPost`

</details>

<details>
<summary><strong>Callbacks, inline, WebApp</strong></summary>

`answerCallbackQuery`, `answerInlineQuery`, `answerWebAppQuery`,
`sendPaidMedia`, `sendChecklist`, `editMessageChecklist`

</details>

<details>
<summary><strong>Extension methods (not in upstream Bot API)</strong></summary>

- `resolveUsername` — resolve `@handle` / `t.me/…` link to
  `{type, chat_id, access_hash}` in one call. No manual peer bootstrap.
- `getMessages` — fetch messages by ID array from a chat.

</details>

**Documented 501 stubs**

- `savePreparedKeyboardButton` / `getPreparedKeyboardButton` — TDLib
  client-side primitives; no MTProto RPC exists. Bot API only exposes them
  because tdbotapi *is* TDLib.
- `sendEphemeralMessage` / `deleteEphemeralMessage` / four
  `editEphemeralMessage*` handlers — MTProto surfaces
  (`EphemeralSendMessage`, `EphemeralDeleteMessage`) landed in gogram after
  the last tagged release (v1.7.71). Unstubs to real handlers once the next
  gogram tag ships.
- `sendChatJoinRequestWebApp` — needs `MessagesRequestChatJoinWebView` from
  the same gogram slice.

---

## Design

### Update pipeline — bypass gogram's dispatcher

Every per-token client is built with `NoUpdates: true` and taps the raw
MTProto transport directly via `client.MTProto.AddCustomServerRequestHandler`.

Why: gogram's own dispatcher runs a PTS/QTS gap detector that misbehaves
under concurrent update-reception + outbound `SendMessage` RPCs. In the tgbotd
setup, the HTTP round-trip latency between "update received" and "handler
calls sendMessage back through the HTTP layer" was interpreted as pts gaps,
causing updates to be buffered indefinitely.

tgbotd unpacks these container types itself: `UpdatesObj`, `UpdatesCombined`,
`UpdateShort`, `UpdateShortMessage`, `UpdateShortChatMessage`. Every inner
update is pushed to a per-bot in-memory buffer, drained by either
`getUpdates` long-poll **or** the webhook deliverer goroutine.

Translator coverage: `message`, `edited_message`, `channel_post`,
`edited_channel_post`, `callback_query` (regular + inline), `inline_query`,
`chosen_inline_result`, `poll`, `poll_answer`, `chat_member`,
`chat_join_request`, `message_reaction`, `message_reaction_count`, plus
business-account and payment-flow variants.

### Architecture

```
HTTP request
    │
    ▼
server/router  ── byte cache for getMe / getMyCommands (served from mem)
    │           ── shared BotInfo cache (name+desc+short_desc = 1 RPC)
    │           ── sync.Pool buffers, HMAC-signed file paths
    │
    ▼
botmgr.Get(token)  ── per-token gogram Client, thread-safe init
    │              ── Prewarm at boot: all persisted bots auth in parallel
    │
    ▼
per-bot gogram Client (NoUpdates:true) ──► MTProto ──► Telegram DC
       │
       ├── SQLite (session, webhook config, file map) — pure-Go modernc driver
       ├── UpdateBuffer (in-memory queue, drained by getUpdates OR webhook)
       └── MTProto.AddCustomServerRequestHandler
              │      taps raw update stream before gogram's PTS/QTS tracker
              ▼
       tlate.UpdateToBotAPI ──► JSON envelope response
```

### Caching layers

| Cache                  | Type                     | Invalidation                                   |
| ---------------------- | ------------------------ | ---------------------------------------------- |
| `getMe`                | pre-encoded JSON bytes   | never (bot identity is immutable)              |
| `getMyCommands`        | pre-encoded JSON bytes / (scope, lang) | any `setMyCommands` / `deleteMyCommands` |
| BotInfo (name/desc/…)  | struct, per lang         | any `setMyName` / `setMyDescription` / `setMyShortDescription` |
| Peer cache             | gogram's own SQLite      | live, MTProto-driven                           |

Each cache uses a generation counter so a concurrent invalidation cannot be
overwritten by an in-flight cold-path write.

### Error mapping

MTProto errors get parsed regardless of how gogram wraps them
(`sending FooRPC: [TAG] human text …`), and mapped to Bot API shape:

| MTProto tag                                        | Bot API code | Notes                          |
| -------------------------------------------------- | ------------ | ------------------------------ |
| `FLOOD_WAIT_*`, `FLOOD_PREMIUM_WAIT_*`, `SLOWMODE_WAIT_*` | 429   | Adds `parameters.retry_after`  |
| `CHAT_WRITE_FORBIDDEN`, `USER_IS_BLOCKED`, `CHAT_ADMIN_REQUIRED`, `CHANNEL_PRIVATE`, `RIGHT_FORBIDDEN`, `USER_BANNED_IN_CHANNEL`, `USER_DEACTIVATED` | 403 | |
| `ACCESS_TOKEN_INVALID/EXPIRED`, `AUTH_KEY_INVALID/UNREGISTERED`, `SESSION_REVOKED` | 401 | |
| everything else                                    | 400          |                                |

### Boot behavior

`Prewarm` at startup reads every bot recorded in SQLite and authenticates
each in a parallel goroutine, so the first HTTP hit for a known token skips
the ~5–6 s MTProto handshake + `LoginBot` + `Me()` cold path. Measured
first-`getMe`: **48 ms** after prewarm vs ~6.5 s without.

### File pipeline

- Uploads: `file_id`, HTTPS URL, `attach://<name>` multipart, or raw multipart
  file. URLs are downloaded server-side to a temp file (Content-Type sniffed
  for extension) then re-uploaded via MTProto.
- Downloads: streamed through `DownloadMedia` with an `io.Pipe`, no full
  buffering.
- `file_id` format: tdlib v4 wire (base64url-nopad + RLE zero-run compression).
- `/file/bot<token>/<path>`: signed via HMAC-SHA256 with a per-bot key derived
  from the bot's token — file paths are single-token-scoped and unforgeable.

### Persistence

One SQLite file (`data/tgbotd.db`, WAL journal) tracks bot rows, webhook
config, and the file id map. Per-bot MTProto session and gogram peer cache
live in `data/bot_<id>.session` and `data/bot_<id>.cache`. All state
survives restarts.

---

## Testing

Everything under `./hack/` is scratch/reproduction code — small, single-file
programs used to validate specific behaviors against a real Telegram account.
None of it is required to run tgbotd.

| Program                 | Purpose                                                  |
| ----------------------- | -------------------------------------------------------- |
| `hack/testbot/`         | Python reference bot polling `getUpdates`, dispatches on `/start`, `/ping`, `/photo`, `/doc`, `/dice`, etc. |
| `hack/live_sweep/`      | 52-step Bot API sweep against a running instance. Prints raw req/resp per step + PASS/FAIL/XFAIL table. |
| `hack/webhook_test/`    | Boots a local HTTP endpoint on `127.0.0.1:9999`, sets it as the webhook target, drives real updates, verifies delivery order + secret token. |
| `hack/autotester/`      | Uses a gogram **user** StringSession to drive a bot end-to-end from a real account. Detects skipped/duplicated updates. |
| `hack/autotester_short/`| Same as above, 4-ping variant. |
| `hack/plain_bot/`       | Reference: pure gogram bot with no HTTP layer. Used to bisect between MTProto/gogram issues and tgbotd HTTP-layer issues. |
| `hack/bench/`           | p50/p95 microbenchmark harness. |
| `hack/trace/`           | Standalone raw-update tracer (tgbotd must be off — same token). |

Each program reads its bot token / StringSession from env vars —
never commit real credentials.

```bash
# 1. Start tgbotd
go run .

# 2. In another shell, run the sweep
TGBOTD_TOKEN=<your-bot-token> go run ./hack/live_sweep
```

---

## Live-verified behaviors

Latest sweep against a real Telegram bot: **49 / 51 methods pass**
end-to-end. The 2 fails are legitimate `FLOOD_WAIT` responses from Telegram
(rate-limiting after repeated `setMyName` calls during testing) — correctly
mapped to HTTP 429 with `parameters.retry_after`.

Verified handling includes: DM chat-member queries (no MTProto participant
concept for user peers), reaction clearing (empty `Reaction` slice),
`sendChatAction` bypassing gogram's typed switch, URL-downloaded photo
uploads with correct tdlib file-id round-trip, and `getMyName` /
`getMyDescription` sharing one RPC via the BotInfo cache.

---

## Non-goals

- **CGO / TDLib.** Everything is pure Go — deploys anywhere Go can
  cross-compile to. That means TDLib-only Bot API primitives (currently
  `savePreparedKeyboardButton` and its `get…` sibling) return a documented
  501 rather than being faked.
- **Configuration files.** Env-only, keeps `go run .` friction-free.
- **Byte-perfect field-for-field parity** on every response object. Type
  shapes match; a handful of rarely-used sub-fields (compound `forward_origin`,
  full `ChatFullInfo`, etc.) are `json.RawMessage`-stubbed and grown per
  demand. Adding a missing field is a 3-line edit to
  `internal/botapi/types.go` and a 2-line translator hook.

---

## Project layout

```
.
├── main.go                     entry point
├── internal/
│   ├── botapi/                 Bot API JSON envelope + type shapes
│   ├── botmgr/                 per-token client, prewarm, webhook deliverer, RPC error mapper
│   ├── config/                 env-var loader
│   ├── logx/                   slog wrapper
│   ├── server/                 HTTP router, ~182 method handlers, /stats, /file
│   ├── storage/                SQLite schema + queries
│   └── tlate/                  MTProto ↔ Bot API translator
├── hack/                       scratch bots / test harnesses (see Testing)
└── data/                       runtime state — .gitignored
```

---

## Status

**v1** — feature-complete against Bot API 10.2 (182 methods + 2 extensions).
Live-verified. Two known 501 stubs are architectural (TDLib-only).

Contributions welcome — this is a spare-time project driven by real-bot
usage. Bug reports with a curl repro are gold.

---

## License

Matches upstream gogram: **GPL-3.0**.
