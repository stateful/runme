package lru

import (
	"container/list"
	"sync"
)

type CacheIdentifier interface {
	Identifier() string
}

type listEntry[T CacheIdentifier] struct {
	id    string
	entry T
}

// Cache is a thread-safe cache of generic entries.
type Cache[T CacheIdentifier] struct {
	capacity int
	mu       sync.RWMutex
	order    *list.List
}

func NewCache[T CacheIdentifier](capacity int) *Cache[T] {
	return &Cache[T]{
		capacity: capacity,
		order:    list.New(),
	}
}

func (sl *Cache[T]) addEntryUnsafe(id string, entry T) {
	if sl.order.Len() >= sl.capacity {
		sl.evictUnsafe()
	}

	sessionEntry := &listEntry[T]{
		id:    id,
		entry: entry,
	}
	sl.order.PushFront(sessionEntry)
}

func (sl *Cache[T]) evictUnsafe() {
	element := sl.order.Back()
	if element != nil {
		sl.order.Remove(element)
	}
}

func (sl *Cache[T]) Add(entry T) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	id := entry.Identifier()
	sl.addEntryUnsafe(id, entry)

	return nil
}

func (sl *Cache[T]) Size() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return sl.order.Len()
}

func (sl *Cache[T]) CreateAndAdd(generate func() (T, error)) (T, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	entry, err := generate()
	if err != nil {
		return entry, err
	}

	id := entry.Identifier()
	sl.addEntryUnsafe(id, entry)
	return entry, nil
}

func (sl *Cache[T]) Get(entry T) (T, bool) {
	id := entry.Identifier()
	return sl.GetByID(id)
}

func (sl *Cache[T]) GetByID(id string) (T, bool) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	for element := sl.order.Front(); element != nil; element = element.Next() {
		sessionEntry := element.Value.(*listEntry[T])
		if sessionEntry.id == id {
			sl.order.MoveToFront(element)
			return sessionEntry.entry, true
		}
	}
	var zero T
	return zero, false
}

func (sl *Cache[T]) Delete(entry T) (present bool) {
	id := entry.Identifier()
	return sl.DeleteByID(id)
}

func (sl *Cache[T]) DeleteByID(id string) (present bool) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	for element := sl.order.Front(); element != nil; element = element.Next() {
		sessionEntry := element.Value.(*listEntry[T])
		if sessionEntry.id == id {
			sl.order.Remove(element)
			return true
		}
	}
	return false
}

func (sl *Cache[T]) Newest() (T, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if sl.order.Len() == 0 {
		var zero T
		return zero, false
	}

	element := sl.order.Front()
	return element.Value.(*listEntry[T]).entry, true
}

func (sl *Cache[T]) MostRecentOrCreate(generate func() (T, error)) (T, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.order.Len() > 0 {
		element := sl.order.Front()
		return element.Value.(*listEntry[T]).entry, nil
	}

	entry, err := generate()
	if err != nil {
		return entry, err
	}

	id := entry.Identifier()
	sl.addEntryUnsafe(id, entry)
	return entry, nil
}

func (sl *Cache[T]) List() []T {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	entries := make([]T, 0, sl.order.Len())
	for element := sl.order.Back(); element != nil; element = element.Prev() {
		entries = append(entries, element.Value.(*listEntry[T]).entry)
	}

	return entries
}
