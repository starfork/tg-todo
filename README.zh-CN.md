# tg-todo
Telegram todo(todolist) bot

[English](README.md)

## 功能
- 把你发给机器人的文本按行解析成 Todo（支持一次发多行批量添加）
- 每个群/私聊各自一份清单（按 chatID 隔离）
- 支持持久化（默认写到 ./db；Docker 场景建议挂载 volume 到 /data）

命令：
- /help 查看帮助
- /list 查看清单
- /done <id> 标记完成
- /undone <id> 取消完成
- /del <id> 删除任务
- /clear 清空清单

新增任务：
- 直接发送文本即可（支持多行），每一行会变成一条任务。
切换完成状态：
- 在私聊中发送 /list，消息下方会出现按钮；点击某条即可切换完成/未完成。

## 运行（本地）
1) 用 BotFather 创建 bot，拿到 token
2) 设置环境变量并启动：

```bash
export TG_TOKEN="xxxxx"
export DB_DIR="./db"
go run .
```

## 群里使用注意
如果你希望 bot 能“看到普通消息并自动按行转 Todo”，需要在 BotFather 关闭隐私模式：
- BotFather -> /setprivacy -> 选择你的 bot -> Disable

然后把 bot 拉进群即可（是否设管理员按你的需要）。

## Docker 部署
### docker compose（推荐）
```bash
export TG_TOKEN="xxxxx"
docker compose up -d --build
```

数据默认落在 `./volumes/tg-todo/`（可按需调整 `docker-compose.yml` 里的 volume）。

### Ubuntu 上如何构建
- Ubuntu x86_64（amd64）上本机构建：直接 `docker compose up -d --build` 或 `docker build -t tg-todo:local .` 即可，不需要 buildx/--platform。
- Ubuntu ARM64（arm64）同理，直接在 ARM64 机器上构建即可得到 ARM64 镜像。

### 指定 Linux 平台构建
如果你遇到类似 “image platform (linux/arm64) does not match host platform (linux/amd64)” 的提示，需要显式指定平台来构建/运行。

构建并加载到本地 Docker：
```bash
docker buildx build --platform linux/amd64 -t tg-todo:local --load .
```

或者构建 ARM64：
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

如果你是按特定平台构建的，运行时也可指定：
```bash
docker run --platform linux/amd64 -d \
  -e TG_TOKEN="xxxxx" \
  -e DB_DIR="/data" \
  -v "$(pwd)/volumes/tg-todo:/data" \
  --name tg-todo \
  tg-todo:local
```

### 导出/导入镜像（tar）
导出：
```bash
docker save tg-todo:local -o tg-todo_local.tar
```

导入：
```bash
docker load -i tg-todo_local.tar
```

如果你把镜像从 Mac 导出 tar 上传到 Ubuntu（amd64）后看到类似告警：
`The requested image's platform (linux/arm64) does not match the detected host platform (linux/amd64/...)`
说明你导入的是 ARM64 镜像（一般是 Apple Silicon 默认产物）。解决方式：
```bash
# Mac 上确认构建的是 amd64
docker buildx build --platform linux/amd64 -t tg-todo:local --load .
docker image inspect tg-todo:local --format '{{.Os}}/{{.Architecture}}'

# 再导出/上传/在 Ubuntu 上导入
docker save tg-todo:local -o tg-todo_local.tar
docker load -i tg-todo_local.tar

# Ubuntu 上也可以检查一次
docker image inspect tg-todo:local --format '{{.Os}}/{{.Architecture}}'
```
使用 docker compose 启动时，为了确保不触发本地构建，可以：
```bash
docker compose up -d --no-build
```

## 配置
- TG_TOKEN（必填）：BotFather 发放的 token
- DB_DIR（可选）：数据目录（默认 ./db；Docker 镜像默认 /data）
- DEBUG（可选）：true/false，开启 go-telegram-bot-api 的 debug 日志
