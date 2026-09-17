# tgira

A task tracker that lives inside a Telegram topic. You write a message, it
becomes a ticket with a number, an owner, a priority and a status. Nothing to
learn, nothing to open, no second place to keep the work.

Self-hosted, single binary, SQLite. Built for a team of three; it will not
mind a few more.

```
🔴 TG-12 · doing
Fix the redirect after the Google login
👤 @ivan · #backend
      [ ✅ Done ]  [ ↩ To do ]
```

## How it works

Every message in a watched topic becomes a task. The bot deletes what you
wrote and posts a card in its place, so the topic reads as a list of tickets
rather than a chat. Replies are left alone — that is where the discussion of a
card goes.

The card number links back to the card itself, so you can drop `TG-12` into
any other topic and land straight on it.

### Writing a task

```
1 @ivan fix the redirect after the Google login #backend
```

| What | How |
|---|---|
| Priority | a lone `1`, `2` or `3` at the start or the end of the first line (`1` is highest) |
| Assignee | the first `@name` |
| Tags | any `#word` |
| Title | the first line of what is left |
| Description | the rest of the lines |

A task can be assigned to somebody the bot has never seen. It holds the handle
and hands the task over the moment that person writes anything.

Send a screenshot with a caption and the attachment is kept: the bot copies it
across and hangs the card underneath.

### Moving a task

Either react to the card or press a button:

| | Reaction | Button |
|---|---|---|
| start working | 👀 | `Take` |
| finished | 👍 (also 🎉 or 💯) | `Done` |
| never mind | 👎 | — |
| back to the queue | remove the reaction | `To do` / `Reopen` |

Telegram allows a fixed set of reaction emoji and a check mark is not in it,
which is why 👍 closes a task rather than ✅.

Taking a reaction back returns the task to the status it had before — the
history knows what that was. A task nobody owns can be picked up by anyone;
once it is owned, only its author and its assignee can move it.

### The board

One pinned message in the topic, rewritten by the bot as things change:

```
📋 Tasks · 4 open

doing
🔴 TG-12 · Redirect after login · @ivan
🟡 TG-9 · Metrics · @petr

to do
🟠 TG-14 · Fix the deploy · @mikita
⚪ TG-13 · Update the readme
```

The circle is the priority: 🔴 1, 🟠 2, 🟡 3, ⚪ none. A closed task takes the
outcome instead: ✅ done, ⚫ cancelled.

## Commands

In the group, where the bot's answer and your command both clear themselves
away after a minute:

| Command | What it does |
|---|---|
| `/whereami` | prints `chat_id` and `thread_id` for the config |
| `/board` | rebuilds and re-pins the board |
| `/tasks [todo\|doing\|done\|cancelled\|all\|@name\|#tag]` | lists tasks, open ones by default |
| `/my` | what is assigned to you |
| `/show TG-1` | the card plus everything that happened to it |
| `/take TG-1` | claim it and start |
| `/assign TG-1 @name` | hand it over |
| `/pri TG-1 2` | change the priority, `-` clears it |
| `/edit TG-1 new text` | rewrite the title, tags and priority |
| `/rm TG-1` | remove it — the author's call alone |
| `/help` | the short version of this page |

In a private chat with the bot, where nothing self-destructs:

| Command | What it does |
|---|---|
| `/start` | lets the bot write to you |
| `/export csv` | `tasks.csv` and `events.csv` |
| `/export json` | one file with the history inside each task |
| `/stats [7d\|30d\|all]` | who created and closed what |

Both are checked against membership of the board's chat.

## Install

### 1. Make the bot

Talk to [@BotFather](https://t.me/BotFather), create a bot, keep the token.
Turn group privacy **off** (`/setprivacy` → Disable), otherwise the bot only
sees commands.

### 2. Put it in the group

Add the bot to the group and **make it an administrator** with the right to
delete messages and pin messages. This is not optional: Telegram only delivers
reactions to administrators, and the bot has to remove the message it turned
into a card.

### 3. Find the topic

Run `/whereami` in the topic you want to track. It answers with the two
numbers the config needs.

### 4. Run it

**systemd**

```sh
make release                      # builds dist/tgira-linux-amd64
scp dist/tgira-linux-amd64 deploy/tgira.service deploy/install.sh \
    config.example.yaml root@your-host:/tmp/tgira/
ssh root@your-host 'cd /tmp/tgira && mv tgira-linux-amd64 tgira && sh install.sh'
```

Then fill in `/etc/tgira/tgira.env` and `/etc/tgira/config.yaml` and start it:

```sh
systemctl enable --now tgira
journalctl -u tgira -f
```

**docker**

```sh
cp config.example.yaml config.yaml   # edit the boards
echo "TGIRA_TOKEN=..." > .env
docker compose up -d
```

## Configuration

`config.yaml`, with every scalar overridable by a `TGIRA_*` environment
variable:

| Key | Env | Default | Meaning |
|---|---|---|---|
| `token` | `TGIRA_TOKEN` | — | the BotFather token |
| `db_path` | `TGIRA_DB_PATH` | `./tgira.db` | where the SQLite file lives |
| `locale` | `TGIRA_LOCALE` | `en` | `en` or `ru` |
| `log_level` | `TGIRA_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `boards[].code` | — | — | the prefix in `TG-12`, uppercase, up to 8 characters |
| `boards[].title` | — | the code | what the board calls itself |
| `boards[].chat_id` | — | — | from `/whereami` |
| `boards[].thread_id` | — | — | from `/whereami` |
| `parse.priority_position` | `TGIRA_PRIORITY_POSITION` | `edges` | `edges` or `anywhere` |
| `ui.buttons` | `TGIRA_BUTTONS` | `true` | buttons under the cards |
| `ui.pin_board` | `TGIRA_PIN_BOARD` | `true` | keep a pinned board |
| `ui.ephemeral_ttl_seconds` | `TGIRA_EPHEMERAL_TTL_SECONDS` | `60` | how long a service message lives |
| `ui.board_debounce_ms` | `TGIRA_BOARD_DEBOUNCE_MS` | `2000` | how long board redraws are pooled |

`priority_position: edges` exists because `fix the 2 buttons on the form`
should be a task about two buttons, not a task with priority 2. Set it to
`anywhere` if you would rather have the digit picked up wherever it stands.

You can watch several topics: add more entries under `boards`. Each keeps its
own numbering and its own board.

## The data

One SQLite file. Copy it and you have moved the tracker; there is nothing else
to carry.

`/export` writes:

- `tasks.csv` — `key, board, num, title, description, status, priority, author, assignee, tags, created_at, updated_at, closed_at, link`
- `events.csv` — `key, actor, kind, from_status, to_status, payload, created_at`
- or one JSON document with the events nested inside each task

Timestamps are RFC 3339 in UTC, so a spreadsheet and pandas both read them
without help.

Nothing is ever really deleted: `/rm` marks a task removed and its history
stays in the export.

## Resources

The binary is around 15 MB and sits at 25–40 MB of memory in use. It runs
comfortably next to other things on the smallest droplet there is.

## Development

```sh
make test     # go test -race ./...
make lint     # go vet + staticcheck
make build
```

The parser, the renderer and the domain rules know nothing about Telegram or
SQL and carry most of the tests. The Telegram layer is tested against a fake
Bot API: updates go in, requests are checked on the way out, and no network is
involved.

To update the golden files after changing what a card looks like:

```sh
go test ./internal/render -update
```
