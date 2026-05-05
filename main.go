package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	token := strings.TrimSpace(os.Getenv("TG_TOKEN"))
	if token == "" {
		log.Fatal("TG_TOKEN is required")
	}

	dbDir := strings.TrimSpace(os.Getenv("DB_DIR"))
	if dbDir == "" {
		dbDir = "./db"
	}

	debug := strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG")), "true")

	store, err := NewFileStore(dbDir)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("init bot: %v", err)
	}
	bot.Debug = debug

	log.Printf("authorized as @%s", bot.Self.UserName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			if update.Message.From == nil {
				continue
			}
			if err := handleMessage(ctx, bot, store, update.Message); err != nil {
				log.Printf("handle message chat=%d user=%d: %v", update.Message.Chat.ID, update.Message.From.ID, err)
			}
		}
	}
}

func handleMessage(ctx context.Context, bot *tgbotapi.BotAPI, store *FileStore, msg *tgbotapi.Message) error {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil
	}

	if msg.IsCommand() {
		return handleCommand(ctx, bot, store, msg)
	}

	lines := splitLines(text)
	if len(lines) == 0 {
		return nil
	}

	list, err := store.Load(ctx, msg.Chat.ID)
	if err != nil {
		return err
	}

	added := 0
	for _, line := range lines {
		parsedText, done, ok := parseTaskLine(line)
		if !ok {
			continue
		}
		list.Add(parsedText, done)
		added++
	}

	if added == 0 {
		return nil
	}

	if err := store.Save(ctx, msg.Chat.ID, list); err != nil {
		return err
	}

	return sendList(bot, msg.Chat.ID, list, fmt.Sprintf("已添加 %d 条：", added))
}

func handleCommand(ctx context.Context, bot *tgbotapi.BotAPI, store *FileStore, msg *tgbotapi.Message) error {
	cmd := strings.ToLower(msg.Command())
	args := strings.TrimSpace(msg.CommandArguments())

	switch cmd {
	case "start", "help":
		return sendText(bot, msg.Chat.ID, helpText())
	case "list":
		list, err := store.Load(ctx, msg.Chat.ID)
		if err != nil {
			return err
		}
		return sendList(bot, msg.Chat.ID, list, "")
	case "add":
		if args == "" {
			return sendText(bot, msg.Chat.ID, "用法：/add 买牛奶")
		}
		lines := splitLines(args)
		list, err := store.Load(ctx, msg.Chat.ID)
		if err != nil {
			return err
		}
		added := 0
		for _, line := range lines {
			parsedText, done, ok := parseTaskLine(line)
			if !ok {
				continue
			}
			list.Add(parsedText, done)
			added++
		}
		if added == 0 {
			return sendText(bot, msg.Chat.ID, "没有可添加的内容。")
		}
		if err := store.Save(ctx, msg.Chat.ID, list); err != nil {
			return err
		}
		return sendList(bot, msg.Chat.ID, list, fmt.Sprintf("已添加 %d 条：", added))
	case "done", "undone", "toggle", "del":
		if args == "" {
			return sendText(bot, msg.Chat.ID, fmt.Sprintf("用法：/%s 3", cmd))
		}
		id, err := strconv.Atoi(strings.Fields(args)[0])
		if err != nil || id <= 0 {
			return sendText(bot, msg.Chat.ID, "任务编号需要是正整数。")
		}
		list, err := store.Load(ctx, msg.Chat.ID)
		if err != nil {
			return err
		}
		var changed bool
		switch cmd {
		case "done":
			changed = list.SetDone(id, true)
		case "undone":
			changed = list.SetDone(id, false)
		case "toggle":
			changed = list.Toggle(id)
		case "del":
			changed = list.Delete(id)
		}
		if !changed {
			return sendText(bot, msg.Chat.ID, "没有找到对应编号的任务。")
		}
		if err := store.Save(ctx, msg.Chat.ID, list); err != nil {
			return err
		}
		return sendList(bot, msg.Chat.ID, list, "")
	case "clear":
		list := NewTodoList()
		if err := store.Save(ctx, msg.Chat.ID, list); err != nil {
			return err
		}
		return sendText(bot, msg.Chat.ID, "已清空。")
	default:
		return sendText(bot, msg.Chat.ID, "未知命令。发送 /help 查看帮助。")
	}
}

func sendList(bot *tgbotapi.BotAPI, chatID int64, list TodoList, title string) error {
	body := list.FormatText()
	if body == "" {
		body = "（空）\n\n发送多行消息即可快速添加任务。"
	}

	var sb strings.Builder
	if strings.TrimSpace(title) != "" {
		sb.WriteString(title)
		sb.WriteString("\n")
	}
	sb.WriteString(body)

	return sendText(bot, chatID, sb.String())
}

func sendText(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	const max = 3900
	text = strings.ReplaceAll(text, "\r\n", "\n")

	if len(text) <= max {
		m := tgbotapi.NewMessage(chatID, text)
		m.DisableWebPagePreview = true
		_, err := bot.Send(m)
		return err
	}

	lines := strings.Split(text, "\n")
	var chunk strings.Builder
	flush := func() error {
		if chunk.Len() == 0 {
			return nil
		}
		m := tgbotapi.NewMessage(chatID, chunk.String())
		m.DisableWebPagePreview = true
		_, err := bot.Send(m)
		chunk.Reset()
		return err
	}

	for _, line := range lines {
		if chunk.Len() == 0 {
			if len(line) > max {
				m := tgbotapi.NewMessage(chatID, line[:max])
				m.DisableWebPagePreview = true
				if _, err := bot.Send(m); err != nil {
					return err
				}
				continue
			}
			chunk.WriteString(line)
			continue
		}

		if chunk.Len()+1+len(line) > max {
			if err := flush(); err != nil {
				return err
			}
			if len(line) > max {
				m := tgbotapi.NewMessage(chatID, line[:max])
				m.DisableWebPagePreview = true
				if _, err := bot.Send(m); err != nil {
					return err
				}
				continue
			}
			chunk.WriteString(line)
			continue
		}
		chunk.WriteString("\n")
		chunk.WriteString(line)
	}

	return flush()
}

func helpText() string {
	return strings.TrimSpace(`
这是一个简单的 Todo Bot：把你发的消息按行拆成待办清单。

命令：
/list        查看清单
/add <text>  添加（支持多行）
/done <id>   标记完成
/undone <id> 取消完成
/toggle <id> 切换完成状态
/del <id>    删除任务
/clear       清空清单

也可以直接发多行文本来添加任务，例如：
买牛奶
写周报
`)
}

func splitLines(s string) []string {
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func parseTaskLine(line string) (text string, done bool, ok bool) {
	s := strings.TrimSpace(line)
	if s == "" {
		return "", false, false
	}

	if strings.HasPrefix(s, "- ") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "- "))
	}

	l := strings.ToLower(s)
	switch {
	case strings.HasPrefix(l, "[x]"):
		return strings.TrimSpace(s[3:]), true, strings.TrimSpace(s[3:]) != ""
	case strings.HasPrefix(l, "[ ]"):
		return strings.TrimSpace(s[3:]), false, strings.TrimSpace(s[3:]) != ""
	default:
		return s, false, true
	}
}
