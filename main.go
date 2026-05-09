package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
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
			if update.CallbackQuery != nil {
				if err := handleCallback(ctx, bot, store, update.CallbackQuery); err != nil {
					chatID := int64(0)
					if update.CallbackQuery.Message != nil && update.CallbackQuery.Message.Chat != nil {
						chatID = update.CallbackQuery.Message.Chat.ID
					}
					userID := int64(0)
					if update.CallbackQuery.From != nil {
						userID = update.CallbackQuery.From.ID
					}
					log.Printf("handle callback chat=%d user=%d: %v", chatID, userID, err)
				}
				continue
			}
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

	storeID := storeIDForMessage(msg)
	if err := store.Ensure(ctx, storeID); err != nil {
		return err
	}

	if msg.IsCommand() {
		return handleCommand(ctx, bot, store, msg, storeID)
	}

	lines := splitLines(text)
	if len(lines) == 0 {
		return nil
	}

	list, err := store.Load(ctx, storeID)
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

	if err := store.Save(ctx, storeID, list); err != nil {
		return err
	}

	return sendList(bot, msg.Chat.ID, list, fmt.Sprintf("已添加 %d 条：", added), msg.Chat != nil && msg.Chat.IsPrivate())
}

func handleCommand(ctx context.Context, bot *tgbotapi.BotAPI, store *FileStore, msg *tgbotapi.Message, storeID int64) error {
	cmd := strings.ToLower(msg.Command())
	args := strings.TrimSpace(msg.CommandArguments())

	switch cmd {
	case "start", "help":
		return sendText(bot, msg.Chat.ID, helpText())
	case "list":
		list, err := store.Load(ctx, storeID)
		if err != nil {
			return err
		}
		return sendList(bot, msg.Chat.ID, list, "", msg.Chat != nil && msg.Chat.IsPrivate())
	case "done", "undone", "del":
		if args == "" {
			return sendText(bot, msg.Chat.ID, fmt.Sprintf("用法：/%s 3", cmd))
		}
		id, err := strconv.Atoi(strings.Fields(args)[0])
		if err != nil || id <= 0 {
			return sendText(bot, msg.Chat.ID, "任务编号需要是正整数。")
		}
		list, err := store.Load(ctx, storeID)
		if err != nil {
			return err
		}
		var changed bool
		switch cmd {
		case "done":
			changed = list.SetDone(id, true)
		case "undone":
			changed = list.SetDone(id, false)
		case "del":
			changed = list.Delete(id)
		}
		if !changed {
			return sendText(bot, msg.Chat.ID, "没有找到对应编号的任务。")
		}
		if err := store.Save(ctx, storeID, list); err != nil {
			return err
		}
		return sendList(bot, msg.Chat.ID, list, "", msg.Chat != nil && msg.Chat.IsPrivate())
	case "clear":
		list := NewTodoList()
		if err := store.Save(ctx, storeID, list); err != nil {
			return err
		}
		return sendText(bot, msg.Chat.ID, "已清空。")
	default:
		return sendText(bot, msg.Chat.ID, "未知命令。发送 /help 查看帮助。")
	}
}

func storeIDForMessage(msg *tgbotapi.Message) int64 {
	if msg.Chat != nil && msg.Chat.IsPrivate() {
		return msg.Chat.ID
	}
	if msg.From == nil {
		return msg.Chat.ID
	}
	return msg.From.ID
}

func sendList(bot *tgbotapi.BotAPI, chatID int64, list TodoList, title string, enableButtons bool) error {
	text := ""
	if enableButtons && len(list.Items) > 0 {
		text = listButtonsHintText(title)
	} else {
		var sb strings.Builder
		if strings.TrimSpace(title) != "" {
			sb.WriteString(title)
			sb.WriteString("\n")
		}
		body := list.FormatText()
		if body == "" {
			body = "（空）\n\n直接发送多行文本即可快速添加任务。"
		}
		sb.WriteString(body)
		text = sb.String()
	}

	m := tgbotapi.NewMessage(chatID, text)
	m.DisableWebPagePreview = true
	if enableButtons {
		m.ReplyMarkup = buildListKeyboard(list)
	}
	_, err := bot.Send(m)
	return err
}

func listButtonsHintText(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "点击条目切换完成状态"
	}
	return title + "\n" + "点击条目切换完成状态"
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

func handleCallback(ctx context.Context, bot *tgbotapi.BotAPI, store *FileStore, q *tgbotapi.CallbackQuery) error {
	if q == nil {
		return nil
	}
	if q.Message == nil || q.Message.Chat == nil {
		return nil
	}
	if q.From == nil {
		return nil
	}

	if !q.Message.Chat.IsPrivate() {
		cb := tgbotapi.NewCallback(q.ID, "请在私聊中使用该按钮。")
		cb.ShowAlert = true
		_, _ = bot.Request(cb)
		return nil
	}

	storeID := storeIDForCallback(q)
	if err := store.Ensure(ctx, storeID); err != nil {
		return err
	}

	itemID, ok := parseToggleCallbackData(q.Data)
	if !ok {
		_, _ = bot.Request(tgbotapi.NewCallback(q.ID, ""))
		return nil
	}

	list, err := store.Load(ctx, storeID)
	if err != nil {
		return err
	}
	if !list.Toggle(itemID) {
		_, _ = bot.Request(tgbotapi.NewCallback(q.ID, ""))
		return nil
	}
	if err := store.Save(ctx, storeID, list); err != nil {
		return err
	}

	body := listButtonsHintText("")
	if len(list.Items) == 0 {
		body = "（空）\n\n直接发送多行文本即可快速添加任务。"
	}

	edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, body)
	edit.DisableWebPagePreview = true
	markup := buildListKeyboard(list)
	edit.ReplyMarkup = &markup
	if _, err := bot.Send(edit); err != nil {
		return err
	}

	_, _ = bot.Request(tgbotapi.NewCallback(q.ID, ""))
	return nil
}

func storeIDForCallback(q *tgbotapi.CallbackQuery) int64 {
	if q.Message != nil && q.Message.Chat != nil && q.Message.Chat.IsPrivate() {
		return q.Message.Chat.ID
	}
	return q.From.ID
}

func parseToggleCallbackData(data string) (id int, ok bool) {
	data = strings.TrimSpace(data)
	if !strings.HasPrefix(data, "t:") {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.TrimPrefix(data, "t:"))
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func buildListKeyboard(list TodoList) tgbotapi.InlineKeyboardMarkup {
	if len(list.Items) == 0 {
		return tgbotapi.NewInlineKeyboardMarkup()
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(list.Items))
	items := make([]TodoItem, 0, len(list.Items))
	items = append(items, list.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	for _, it := range items {
		suffix := ""
		if it.Done {
			suffix = " √"
		}
		label := fmt.Sprintf("%d. %s%s", it.ID, truncateForButton(it.Text, 40), suffix)
		toggleBtn := tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("t:%d", it.ID))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(toggleBtn))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func truncateForButton(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

func helpText() string {
	return strings.TrimSpace(`
这是一个简单的 Todo Bot：把你发的消息按行拆成待办清单。

命令：
/list        查看清单
/done <id>   标记完成
/undone <id> 取消完成
/del <id>    删除任务
/clear       清空清单

也可以直接发多行文本来添加任务，例如：
买牛奶
写周报

在私聊里，/list 发出来的清单支持点击条目来切换完成状态。
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
