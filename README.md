# tgbotd

> A from-scratch **Telegram Bot API server** written in pure Go — drop-in for
> `telegram-bot-api`, backed directly by MTProto via
> [gogram](https://github.com/amarnathcjd/gogram).

Point any Bot API client at `http://127.0.0.1:8081/bot<TOKEN>/<method>` and
it just works. No TDLib, no CGO, single static binary. Bot API surface up to
**10.2** — 182 methods wired, plus 2 extension methods (`resolveUsername`,
`getMessages`).

## Quick start

```bash
git clone https://github.com/AmarnathCJD/tgbotd
cd tgbotd
go run .
```

```bash
curl -X POST "http://127.0.0.1:8081/bot$TELEGRAM_BOT_TOKEN/getMe"
```

Ships with public MTProto `api_id`/`api_hash` defaults — first call
authenticates, caches session under `./data/`, later calls reuse it.

## Benchmark vs `api.telegram.org`

Serial p50 latency, n=20, 5-request warm-up:

| method                          | tgbotd      | upstream | speedup   |
| ------------------------------- | ----------- | -------- | --------- |
| `getMe`                         | **0.55 ms** | 150 ms   | **~275×** |
| `getMyCommands` (cached)        | **0.5 ms**  | 152 ms   | **~300×** |
| `getWebhookInfo`                | **0.54 ms** | 153 ms   | **~285×** |
| `getUpdates` (short-poll, idle) | **0 ms**    | 3140 ms  | **∞**     |
| `sendMessage`                   | 210 ms      | 240 ms   | ≈1.1×     |

Cache-friendly reads stay sub-millisecond; network-bound writes hit the DC
RTT floor. Under 20-concurrent load tgbotd holds 1–4 ms p50 with zero errors
while upstream starts throwing 429s.

## Configuration

Environment only — no config files.

| Variable            | Default          | Purpose                              |
| ------------------- | ---------------- | ------------------------------------ |
| `TGBOTD_HTTP_ADDR`  | `127.0.0.1:8081` | HTTP listen address                  |
| `TGBOTD_DATA_DIR`   | `./data`         | Session / cache / SQLite state       |
| `TGBOTD_API_ID`     | `2040`           | MTProto app id                       |
| `TGBOTD_API_HASH`   | (baked default)  | MTProto app hash                     |
| `TGBOTD_TEST_DC`    | `false`          | Route to Telegram test DCs           |
| `TGBOTD_LOG_LEVEL`  | `info`           | `debug` / `info` / `warn` / `error`  |

## Endpoints

| Path                             | Method  | Description                                        |
| -------------------------------- | ------- | -------------------------------------------------- |
| `/bot<token>/<method>`           | POST    | Bot API method dispatch                            |
| `/file/bot<token>/<path>`        | GET     | Stream a file (HMAC-verified, per-bot signed path) |
| `/stats`                         | GET     | Per-method call counts + p50/p99 latency           |

## Design notes

- **Bypasses gogram's dispatcher.** Each per-token client runs with
  `NoUpdates: true` and taps the raw MTProto transport via
  `client.MTProto.AddCustomServerRequestHandler`. Gogram's PTS/QTS gap
  detector otherwise misreads HTTP round-trip latency as pts gaps and stalls
  update delivery.
- **Byte caches** for `getMe` and `getMyCommands`; shared `BotInfo` cache
  so `getMyName` + `getMyDescription` + `getMyShortDescription` share one
  MTProto call. Each cache uses a generation counter for race-free
  invalidation.
- **URL uploads** are downloaded server-side, re-uploaded via MTProto, and
  round-tripped back as tdlib-v4 file ids. Telegram's OG-tag crawler is
  bypassed, so any HTTPS host works.
- **File paths** are HMAC-SHA256-signed with a per-bot key — file ids can't
  be replayed against another bot.
- **Prewarm at startup**: every persisted bot re-authenticates in parallel,
  so the first HTTP hit is ~48 ms instead of ~6 s.
- **Webhook deliverer** POSTs one update at a time with 1 s → 300 s
  exponential backoff, and drops an individual update after 8 attempts so a
  broken endpoint can't wedge the pipeline.
- **Error mapping.** MTProto tags are extracted regardless of gogram's
  wrapping (`sending Foo: [TAG] …`) and mapped to Bot API shape:
  `FLOOD_WAIT_*` → 429 with `retry_after`, chat-access errors → 403, auth
  errors → 401.
- **Persistence.** One SQLite file (`data/tgbotd.db`, WAL) + per-bot session
  files. Everything survives restarts.

## Documented 501 stubs

- `savePreparedKeyboardButton` / `getPreparedKeyboardButton` — TDLib
  client-side primitives; no MTProto RPC exists.
- `sendEphemeralMessage` / `deleteEphemeralMessage` (and 4
  `editEphemeralMessage*` handlers), `sendChatJoinRequestWebApp` — MTProto
  surfaces landed in gogram after v1.7.71; unstub when the next gogram tag
  ships.

## Examples

`./examples/` holds small, single-file programs used to validate behavior
against real Telegram. None are required to run tgbotd.

| Program                     | Purpose                                                |
| --------------------------- | ------------------------------------------------------ |
| `examples/testbot/`         | Python reference bot polling `getUpdates`, handles `/start`, `/ping`, `/photo`, `/doc`, `/dice` |
| `examples/live_sweep/`      | 52-step Bot API sweep. Raw req/resp per step + PASS/FAIL table |
| `examples/webhook_test/`    | Local HTTP endpoint on `127.0.0.1:9999`, verifies delivery order + secret token |
| `examples/autotester/`      | Uses a gogram user StringSession to drive a bot end-to-end from a real account |
| `examples/plain_bot/`       | Pure-gogram bot with no HTTP layer — used to bisect MTProto vs. HTTP-layer issues |
| `examples/bench/`           | p50 / p95 microbenchmark harness |

Every example reads its bot token / StringSession from env — never commit
real credentials.

```bash
go run .                                              # start tgbotd
TGBOTD_TOKEN=<bot-token> go run ./examples/live_sweep # sweep against it
```

## License

GPL-3.0, matching upstream gogram.
