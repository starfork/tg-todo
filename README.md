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
- /done <id> mark as done
- /undone <id> mark as not done
- /del <id> delete an item
- /clear clear the list

Adding items:
- Send text directly (multi-line supported). Each line becomes a todo.
Toggling done:
- In private chat, send /list. You will see buttons under the message; tap one item to toggle its done state.

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

### How to build on Ubuntu
- Ubuntu x86_64 (amd64): just run `docker compose up -d --build` or `docker build -t tg-todo:local .`. No buildx / `--platform` needed.
- Ubuntu ARM64 (arm64): same idea — build on the ARM64 machine to get an ARM64 image.

### Build for a specific Linux platform
If you see an error like "image platform (linux/arm64) does not match host platform (linux/amd64)", build and run with an explicit platform.

Build and load into local Docker:
```bash
docker buildx build --platform linux/amd64 -t tg-todo:local --load .
```

Or for ARM64:
```bash
docker buildx build --platform linux/arm64 -t tg-todo:local --load .
```

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

If you built for a specific platform, run with:
```bash
docker run --platform linux/amd64 -d \
  -e TG_TOKEN="xxxxx" \
  -e DB_DIR="/data" \
  -v "$(pwd)/volumes/tg-todo:/data" \
  --name tg-todo \
  tg-todo:local
```

### Export / Import image (tar)
Export:
```bash
docker save tg-todo:local -o tg-todo_local.tar
```

Import:
```bash
docker load -i tg-todo_local.tar
```

If you export a tar from Mac and import it on Ubuntu (amd64) and see a warning like:
`The requested image's platform (linux/arm64) does not match the detected host platform (linux/amd64/...)`
it means you imported an ARM64 image (often the default output on Apple Silicon). Fix by building amd64 on Mac before exporting:
```bash
docker buildx build --platform linux/amd64 -t tg-todo:local --load .
docker image inspect tg-todo:local --format '{{.Os}}/{{.Architecture}}'
docker save tg-todo:local -o tg-todo_local.tar
```
Then on Ubuntu:
```bash
docker load -i tg-todo_local.tar
docker image inspect tg-todo:local --format '{{.Os}}/{{.Architecture}}'
docker compose up -d --no-build
```

## Configuration
- TG_TOKEN (required): token from BotFather
- DB_DIR (optional): data directory (default `./db`; Docker image default `/data`)
- DEBUG (optional): true/false, enables go-telegram-bot-api debug logs
