package pastehub

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

type Item struct {
	BufferName  string    `json:"buffer_name"`
	ItemName    string    `json:"item_name,omitempty"`
	ContentType string    `json:"content_type"`
	Size        int       `json:"size"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
	Data        []byte    `json:"-"`
}

type Store struct {
	mu      sync.RWMutex
	buffers map[string]Item
}

func NewStore() *Store {
	return &Store{buffers: make(map[string]Item)}
}

func (s *Store) Put(bufferName, itemName, contentType string, data []byte) Item {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	hash := sha256.Sum256(data)
	item := Item{
		BufferName:  bufferName,
		ItemName:    itemName,
		ContentType: contentType,
		Size:        len(data),
		SHA256:      hex.EncodeToString(hash[:]),
		CreatedAt:   time.Now().UTC(),
		Data:        append([]byte(nil), data...),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffers[bufferName] = item
	return item
}

func (s *Store) Get(bufferName string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.buffers[bufferName]
	if !ok {
		return Item{}, false
	}
	item.Data = append([]byte(nil), item.Data...)
	return item, true
}

func (s *Store) Delete(bufferName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.buffers[bufferName]
	if ok {
		delete(s.buffers, bufferName)
	}
	return ok
}

func (s *Store) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Item, 0, len(s.buffers))
	for _, item := range s.buffers {
		item.Data = nil
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].BufferName < items[j].BufferName
	})
	return items
}
