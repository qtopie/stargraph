package memory

import (
	"context"
	"fmt"
	"sync"
)

// KV 纯内存并发安全的 Key-Value 存储
type KV struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewKV 创建内存 KV 存储实例
func NewKV() *KV {
	return &KV{
		data: make(map[string][]byte),
	}
}

func (m *KV) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, exists := m.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	res := make([]byte, len(val))
	copy(res, val)
	return res, nil
}

func (m *KV) Set(ctx context.Context, key string, val []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := make([]byte, len(val))
	copy(cp, val)
	m.data[key] = cp
	return nil
}

func (m *KV) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

func (m *KV) BatchGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string][]byte, len(keys))
	for _, k := range keys {
		if val, ok := m.data[k]; ok {
			cp := make([]byte, len(val))
			copy(cp, val)
			res[k] = cp
		}
	}
	return res, nil
}

func (m *KV) BatchSet(ctx context.Context, kvMap map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range kvMap {
		cp := make([]byte, len(v))
		copy(cp, v)
		m.data[k] = cp
	}
	return nil
}

func (m *KV) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	return nil
}
