package churn

import (
	"container/list"
	"sync"

	"github.com/alexcollie/kaizen/pkg/models"
)

// churnCache is a small LRU of churn metrics keyed by repo-relative path
type churnCache struct {
	mutex    sync.RWMutex
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

type cacheEntry struct {
	key    string
	metric *models.ChurnMetric
}

// newChurnCache creates a cache holding at most capacity entries
func newChurnCache(capacity int) *churnCache {
	return &churnCache{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[string]*list.Element, capacity),
	}
}

// Get returns the cached metric for key and marks it recently used
func (cache *churnCache) Get(key string) (*models.ChurnMetric, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	element, ok := cache.entries[key]
	if !ok {
		return nil, false
	}
	cache.order.MoveToFront(element)
	return element.Value.(*cacheEntry).metric, true
}

// Put stores metric under key, evicting the least recently used entry when full
func (cache *churnCache) Put(key string, metric *models.ChurnMetric) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if element, ok := cache.entries[key]; ok {
		element.Value.(*cacheEntry).metric = metric
		cache.order.MoveToFront(element)
		return
	}

	element := cache.order.PushFront(&cacheEntry{key: key, metric: metric})
	cache.entries[key] = element

	if cache.order.Len() > cache.capacity {
		oldest := cache.order.Back()
		cache.order.Remove(oldest)
		delete(cache.entries, oldest.Value.(*cacheEntry).key)
	}
}

// Len reports how many entries are cached
func (cache *churnCache) Len() int {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	return cache.order.Len()
}
