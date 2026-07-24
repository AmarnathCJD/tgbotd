#!/usr/bin/env python3
"""End-to-end test bot for tgbotd.

Talks to a local tgbotd instance via the Bot API HTTP protocol. Registers
handlers for every category so a human tester can exercise the server live.

Handlers registered:

Commands:
    /start          — welcome + inline keyboard demo
    /help           — list all commands
    /ping           — round-trip time
    /me             — echo the user object (proves From / chat lookups)
    /photo          — send a photo by URL
    /doc            — send a document by URL
    /location       — send a fixed location
    /venue          — send a venue
    /contact        — send a contact
    /poll           — send an interactive poll
    /quiz           — send a quiz (poll with correct answer)
    /dice           — send a dice
    /keyboard       — show a reply keyboard
    /remove         — remove the reply keyboard
    /forceReply     — force-reply demo
    /markdown       — markdown formatting test
    /html           — HTML formatting test
    /reactions      — set a reaction on the previous message
    /resolve <name> — extension method: resolveUsername
    /chat           — show current chat info (getChat)
    /me_full        — showcases /getMe

Media:
    Any photo you send  → bot echoes it back + shows file_id
    Any document        → bot echoes filename + size
    Any location        → bot echoes coordinates

Text:
    Any non-command text → bot echoes it with a "you said:" prefix

Callback queries:
    Buttons under /start and /keyboard have callback_data; bot answers them.

Inline mode:
    @<botname> anything → returns a text article result
"""
import json
import os
import sys
import time
from urllib.parse import urlencode
from urllib.request import Request, urlopen
from urllib.error import HTTPError

BASE = os.environ.get("TGBOTD_URL", "http://127.0.0.1:8081")
TOKEN = os.environ.get("TGBOTD_TOKEN")
if not TOKEN:
    raise SystemExit("TGBOTD_TOKEN env var is required")


def call(method: str, **params) -> dict:
    """POST a Bot API method call to tgbotd and return the parsed JSON body."""
    url = f"{BASE}/bot{TOKEN}/{method}"
    body = json.dumps(params, separators=(",", ":")).encode("utf-8")
    req = Request(url, data=body, headers={"Content-Type": "application/json"})
    try:
        with urlopen(req, timeout=65) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except HTTPError as e:
        try:
            return json.loads(e.read().decode("utf-8"))
        except Exception:
            return {"ok": False, "error_code": e.code, "description": str(e)}


def send_message(chat_id, text, **extra):
    return call("sendMessage", chat_id=chat_id, text=text, **extra)


def answer_callback(cbq_id, text, alert=False):
    return call("answerCallbackQuery", callback_query_id=cbq_id, text=text, show_alert=alert)


def answer_inline(query_id, results):
    return call("answerInlineQuery", inline_query_id=query_id, results=results, cache_time=1)


# ---------- command handlers ----------

def cmd_start(msg):
    chat_id = msg["chat"]["id"]
    kb = {
        "inline_keyboard": [
            [
                {"text": "Ping", "callback_data": "ping"},
                {"text": "Show help", "callback_data": "help"},
            ],
            [
                {"text": "Open Telegram", "url": "https://t.me/telegram"},
                {"text": "Random dice", "callback_data": "dice"},
            ],
        ]
    }
    send_message(
        chat_id,
        "*tgbotd test bot online.*\n\nTry /help to see everything.",
        parse_mode="Markdown",
        reply_markup=kb,
    )


def cmd_help(msg):
    text = (
        "Commands wired end-to-end via tgbotd:\n\n"
        "/start /help /ping /me /me_full /chat\n"
        "/photo /doc /location /venue /contact\n"
        "/poll /quiz /dice\n"
        "/keyboard /remove /forceReply\n"
        "/markdown /html /reactions\n"
        "/resolve <username> — extension method\n\n"
        "Plus: text echo, media echo, callback answers, inline mode."
    )
    send_message(msg["chat"]["id"], text)


def cmd_ping(msg):
    t0 = time.time()
    r = call("getMe")
    dt = (time.time() - t0) * 1000
    send_message(msg["chat"]["id"], f"getMe round-trip: {dt:.2f} ms — {r.get('result',{}).get('username','?')}")


def cmd_me(msg):
    u = msg.get("from") or {}
    text = (
        f"Your data as seen by tgbotd:\n"
        f"id: {u.get('id')}\n"
        f"is_bot: {u.get('is_bot')}\n"
        f"first_name: {u.get('first_name')}\n"
        f"last_name: {u.get('last_name','')}\n"
        f"username: @{u.get('username','')}\n"
        f"language_code: {u.get('language_code','')}\n"
        f"chat.type: {msg['chat']['type']}"
    )
    send_message(msg["chat"]["id"], text)


def cmd_me_full(msg):
    r = call("getMe")
    send_message(msg["chat"]["id"], f"getMe:\n{json.dumps(r.get('result'), indent=2)}")


def cmd_chat(msg):
    r = call("getChat", chat_id=msg["chat"]["id"])
    send_message(msg["chat"]["id"], f"getChat:\n{json.dumps(r.get('result'), indent=2)}")


def cmd_photo(msg):
    call("sendPhoto",
         chat_id=msg["chat"]["id"],
         photo="https://picsum.photos/600/400",
         caption="via tgbotd sendPhoto (URL upload)")


def cmd_doc(msg):
    r = call("sendDocument",
             chat_id=msg["chat"]["id"],
             document="https://raw.githubusercontent.com/torvalds/linux/master/README",
             caption="sendDocument by URL")
    if not r.get("ok"):
        send_message(msg["chat"]["id"], f"sendDocument error: {r.get('description')}")


def cmd_location(msg):
    call("sendLocation", chat_id=msg["chat"]["id"], latitude=37.7749, longitude=-122.4194)


def cmd_venue(msg):
    call("sendVenue",
         chat_id=msg["chat"]["id"],
         latitude=48.8566, longitude=2.3522,
         title="Eiffel Tower", address="Champ de Mars, Paris")


def cmd_contact(msg):
    call("sendContact",
         chat_id=msg["chat"]["id"],
         phone_number="+15551234567", first_name="tgbotd", last_name="demo")


def cmd_poll(msg):
    call("sendPoll",
         chat_id=msg["chat"]["id"],
         question="Is tgbotd faster than the official server?",
         options=["yes, way faster", "obviously", "of course"],
         is_anonymous=False)


def cmd_quiz(msg):
    call("sendPoll",
         chat_id=msg["chat"]["id"],
         question="What language is tgbotd written in?",
         options=["Rust", "Go", "C++", "Python"],
         type="quiz",
         correct_option_ids=[1],
         explanation="tgbotd is written in Go, using gogram for MTProto.")


def cmd_dice(msg):
    call("sendDice", chat_id=msg["chat"]["id"], emoji="🎲")


def cmd_keyboard(msg):
    kb = {
        "keyboard": [
            [{"text": "A"}, {"text": "B"}, {"text": "C"}],
            [{"text": "Send my location", "request_location": True}],
            [{"text": "Send my contact", "request_contact": True}],
        ],
        "resize_keyboard": True,
        "one_time_keyboard": False,
    }
    send_message(msg["chat"]["id"], "reply keyboard shown — tap a button", reply_markup=kb)


def cmd_remove(msg):
    send_message(msg["chat"]["id"], "reply keyboard removed", reply_markup={"remove_keyboard": True})


def cmd_forcereply(msg):
    send_message(msg["chat"]["id"], "Reply to this message", reply_markup={"force_reply": True})


def cmd_markdown(msg):
    text = "*bold* _italic_ __underline__ ~strike~ ||spoiler|| `code` [link](https://telegram.org)"
    send_message(msg["chat"]["id"], text, parse_mode="MarkdownV2")


def cmd_html(msg):
    text = "<b>bold</b> <i>italic</i> <u>underline</u> <s>strike</s> <tg-spoiler>spoiler</tg-spoiler> <code>code</code> <a href='https://telegram.org'>link</a>"
    send_message(msg["chat"]["id"], text, parse_mode="HTML")


def cmd_rich(msg):
    """Rich Messages test — sends via the /sendRichMessage extension."""
    rich = {
        "html": (
            "<b>tgbotd rich message</b><br/>"
            "<i>this is italic</i><br/>"
            "<a href='https://telegram.org'>Telegram</a>"
        ),
    }
    r = call("sendRichMessage",
             chat_id=msg["chat"]["id"],
             rich_message=rich)
    send_message(msg["chat"]["id"], f"sendRichMessage response: {json.dumps(r)}")


def cmd_richdraft(msg):
    rich = {"html": "<b>draft rich</b> <i>preview</i>"}
    r = call("sendRichMessageDraft", chat_id=msg["chat"]["id"], rich_message=rich)
    send_message(msg["chat"]["id"], f"sendRichMessageDraft: {json.dumps(r)}")


def cmd_reactions(msg):
    reply_to = msg.get("reply_to_message")
    target_id = (reply_to or msg)["message_id"]
    call("setMessageReaction",
         chat_id=msg["chat"]["id"],
         message_id=target_id,
         reaction=[{"type": "emoji", "emoji": "🔥"}])


def cmd_resolve(msg, args):
    if not args:
        send_message(msg["chat"]["id"], "usage: /resolve <username>")
        return
    r = call("resolveUsername", username=args)
    send_message(msg["chat"]["id"], f"resolveUsername:\n{json.dumps(r, indent=2)}")


COMMANDS = {
    "start": cmd_start,
    "help": cmd_help,
    "ping": cmd_ping,
    "me": cmd_me,
    "me_full": cmd_me_full,
    "chat": cmd_chat,
    "photo": cmd_photo,
    "doc": cmd_doc,
    "location": cmd_location,
    "venue": cmd_venue,
    "contact": cmd_contact,
    "poll": cmd_poll,
    "quiz": cmd_quiz,
    "dice": cmd_dice,
    "keyboard": cmd_keyboard,
    "remove": cmd_remove,
    "forcereply": cmd_forcereply,
    "markdown": cmd_markdown,
    "html": cmd_html,
    "reactions": cmd_reactions,
    "rich": cmd_rich,
    "richdraft": cmd_richdraft,
}


# ---------- update dispatcher ----------

def handle_message(msg):
    text = msg.get("text", "")
    chat_id = msg["chat"]["id"]

    if text.startswith("/"):
        parts = text[1:].split(None, 1)
        cmd = parts[0].split("@")[0].lower()
        args = parts[1] if len(parts) > 1 else ""
        if cmd == "resolve":
            cmd_resolve(msg, args)
            return
        h = COMMANDS.get(cmd)
        if h:
            h(msg)
        else:
            send_message(chat_id, f"unknown command: /{cmd}")
        return

    if "photo" in msg:
        sizes = msg["photo"]
        biggest = sizes[-1]
        send_message(chat_id,
                     f"got photo — file_id: {biggest['file_id'][:32]}...\nsize: {biggest.get('file_size','?')} bytes\ndims: {biggest['width']}x{biggest['height']}")
        return

    if "document" in msg:
        d = msg["document"]
        send_message(chat_id, f"got document — name: {d.get('file_name','')}, mime: {d.get('mime_type','')}, size: {d.get('file_size','?')}")
        return

    if "location" in msg:
        loc = msg["location"]
        send_message(chat_id, f"got location — {loc['latitude']:.5f}, {loc['longitude']:.5f}")
        return

    if text:
        send_message(chat_id, f"you said: {text}")


def handle_callback(cbq):
    data = cbq.get("data", "")
    from_id = cbq["from"]["id"]

    if data == "ping":
        answer_callback(cbq["id"], "pong")
    elif data == "help":
        answer_callback(cbq["id"], "opening help...")
        if cbq.get("message"):
            cmd_help(cbq["message"])
    elif data == "dice":
        answer_callback(cbq["id"], "rolling...")
        if cbq.get("message"):
            call("sendDice", chat_id=cbq["message"]["chat"]["id"], emoji="🎲")
    else:
        answer_callback(cbq["id"], f"clicked: {data}")


def handle_inline(iq):
    query = iq.get("query", "")
    results = [
        {
            "type": "article",
            "id": "1",
            "title": "Echo",
            "description": f"Send: {query or '(empty)'}",
            "input_message_content": {
                "message_text": f"You searched for: {query or '(empty)'}",
            },
        },
        {
            "type": "article",
            "id": "2",
            "title": "tgbotd status",
            "description": "click to send status message",
            "input_message_content": {
                "message_text": "tgbotd is running fine.",
            },
        },
    ]
    answer_inline(iq["id"], results)


def main():
    r = call("getMe")
    if not r.get("ok"):
        print("failed to reach tgbotd:", r, file=sys.stderr)
        sys.exit(1)
    bot = r["result"]
    print(f"connected as @{bot['username']} (id {bot['id']})", flush=True)

    call("setMyCommands", commands=[{"command": c, "description": c} for c in COMMANDS.keys()])

    offset = 0
    while True:
        r = call("getUpdates", offset=offset, timeout=25, allowed_updates=[
            "message", "callback_query", "inline_query",
        ])
        if not r.get("ok"):
            print("getUpdates error:", r, file=sys.stderr)
            time.sleep(1)
            continue
        for u in r.get("result", []):
            offset = u["update_id"] + 1
            try:
                if "message" in u:
                    handle_message(u["message"])
                elif "callback_query" in u:
                    handle_callback(u["callback_query"])
                elif "inline_query" in u:
                    handle_inline(u["inline_query"])
            except Exception as e:
                import traceback
                print(f"handler error: {e}", file=sys.stderr)
                traceback.print_exc()
                try:
                    if "message" in u:
                        send_message(u["message"]["chat"]["id"], f"handler error: {e}")
                except Exception:
                    pass


if __name__ == "__main__":
    main()
