package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type TodoItem struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type TodoList struct {
	NextID int        `json:"next_id"`
	Items  []TodoItem `json:"items"`
}

func NewTodoList() TodoList {
	return TodoList{NextID: 1, Items: nil}
}

func (l *TodoList) Add(text string, done bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if l.NextID <= 0 {
		l.NextID = 1
	}
	l.Items = append(l.Items, TodoItem{
		ID:        l.NextID,
		Text:      text,
		Done:      done,
		CreatedAt: time.Now().UTC(),
	})
	l.NextID++
}

func (l *TodoList) SetDone(id int, done bool) bool {
	for i := range l.Items {
		if l.Items[i].ID == id {
			l.Items[i].Done = done
			return true
		}
	}
	return false
}

func (l *TodoList) Toggle(id int) bool {
	for i := range l.Items {
		if l.Items[i].ID == id {
			l.Items[i].Done = !l.Items[i].Done
			return true
		}
	}
	return false
}

func (l *TodoList) Delete(id int) bool {
	for i := range l.Items {
		if l.Items[i].ID == id {
			l.Items = append(l.Items[:i], l.Items[i+1:]...)
			return true
		}
	}
	return false
}

func (l TodoList) FormatText() string {
	if len(l.Items) == 0 {
		return ""
	}

	items := make([]TodoItem, 0, len(l.Items))
	items = append(items, l.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	var sb strings.Builder
	for _, it := range items {
		box := "☐"
		if it.Done {
			box = "✅"
		}
		sb.WriteString(fmt.Sprintf("%s %d. %s\n", box, it.ID, it.Text))
	}
	return strings.TrimRight(sb.String(), "\n")
}

type FileStore struct {
	dir string

	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func NewFileStore(dir string) (*FileStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("empty dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir, locks: make(map[int64]*sync.Mutex)}, nil
}

func (s *FileStore) Ensure(ctx context.Context, chatID int64) error {
	_ = ctx
	lock := s.chatLock(chatID)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	path := s.pathForChat(chatID)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	return s.saveLocked(chatID, NewTodoList())
}

func (s *FileStore) Load(ctx context.Context, chatID int64) (TodoList, error) {
	_ = ctx
	lock := s.chatLock(chatID)
	lock.Lock()
	defer lock.Unlock()

	path := s.pathForChat(chatID)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewTodoList(), nil
		}
		return TodoList{}, err
	}
	var list TodoList
	if err := json.Unmarshal(b, &list); err != nil {
		return NewTodoList(), nil
	}
	if list.NextID <= 0 {
		maxID := 0
		for _, it := range list.Items {
			if it.ID > maxID {
				maxID = it.ID
			}
		}
		list.NextID = maxID + 1
		if list.NextID <= 0 {
			list.NextID = 1
		}
	}
	return list, nil
}

func (s *FileStore) Save(ctx context.Context, chatID int64, list TodoList) error {
	_ = ctx
	lock := s.chatLock(chatID)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	return s.saveLocked(chatID, list)
}

func (s *FileStore) saveLocked(chatID int64, list TodoList) error {
	path := s.pathForChat(chatID)

	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(s.dir, fmt.Sprintf("chat_%d_*.tmp", chatID))
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}

func (s *FileStore) pathForChat(chatID int64) string {
	return filepath.Join(s.dir, fmt.Sprintf("chat_%d.json", chatID))
}

func (s *FileStore) chatLock(chatID int64) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks == nil {
		s.locks = make(map[int64]*sync.Mutex)
	}
	if m, ok := s.locks[chatID]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.locks[chatID] = m
	return m
}
