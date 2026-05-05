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
- /add <text> 添加（支持多行）
- /done <id> 标记完成
- /undone <id> 取消完成
- /toggle <id> 切换完成状态
- /del <id> 删除任务
- /clear 清空清单

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

## 配置
- TG_TOKEN（必填）：BotFather 发放的 token
- DB_DIR（可选）：数据目录（默认 ./db；Docker 镜像默认 /data）
- DEBUG（可选）：true/false，开启 go-telegram-bot-api 的 debug 日志

