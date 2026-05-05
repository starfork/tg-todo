# tg-todo
Telegram todo(todolist) bot (Go).

[中文说明](README.zh-CN.md)

## Features
- Turns your messages into a checklist by parsing text line-by-line (send multi-line messages to batch add)
- One list per chat (DM / group), isolated by chatID
- File-based persistence (defaults to `./db`; recommended to mount a volume to `/data` in Docker)

Commands:
- /help show help
- /list show current list
- /add <text> add items (multi-line supported)
- /done <id> mark as done
- /undone <id> mark as not done
- /toggle <id> toggle done status
- /del <id> delete an item
- /clear clear the list

## Run locally
1) Create a bot via BotFather and get the token
2) Set env vars and start:

```bash
export TG_TOKEN="xxxxx"
export DB_DIR="./db"
go run .
```

## Group usage note
If you want the bot to receive regular (non-command) messages in groups and auto-convert them to todos, disable privacy mode in BotFather:
- BotFather -> /setprivacy -> select your bot -> Disable

Then add the bot to your group (admin is optional depending on your needs).

## Docker
### docker compose (recommended)
```bash
export TG_TOKEN="xxxxx"
docker compose up -d --build
```

Data is stored in `./volumes/tg-todo/` by default (adjust the volume mapping in `docker-compose.yml` if needed).

### docker run
```bash
docker build -t tg-todo:local .
docker run -d \
  -e TG_TOKEN="xxxxx" \
  -e DB_DIR="/data" \
  -v "$(pwd)/volumes/tg-todo:/data" \
  --name tg-todo \
  tg-todo:local
```

## Configuration
- TG_TOKEN (required): token from BotFather
- DB_DIR (optional): data directory (default `./db`; Docker image default `/data`)
- DEBUG (optional): true/false, enables go-telegram-bot-api debug logs
