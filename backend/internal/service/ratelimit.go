package service

import (
	"context"
	"sync"
	"time"

	"github.com/xmtrxi/edms/internal/cache"
)

// Limiter counts attempts per key within a window. It uses Valkey so every
// API instance shares the count, and falls back to memory without it.
type Limiter struct {
	cache *cache.Cache
	mu    sync.Mutex
	local map[string]*limiterWindow
}

type limiterWindow struct {
	count   int64
	expires time.Time
}

func NewLimiter(c *cache.Cache) *Limiter {
	return &Limiter{cache: c, local: map[string]*limiterWindow{}}
}

// Count reports how many attempts the key has made in its current window.
func (l *Limiter) Count(ctx context.Context, key string) int64 {
	if l.cache != nil {
		value, err := l.cache.Client.Do(ctx, l.cache.Client.B().Get().Key(limiterKey(key)).Build()).AsInt64()
		if err == nil {
			return value
		}
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if w := l.local[key]; w != nil && time.Now().Before(w.expires) {
		return w.count
	}
	return 0
}

// Hit records an attempt and returns the count in the window.
func (l *Limiter) Hit(ctx context.Context, key string, window time.Duration) int64 {
	if l.cache != nil {
		count, err := l.cache.IncrementWindow(ctx, limiterKey(key), window)
		if err == nil {
			return count
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	w := l.local[key]
	if w == nil || now.After(w.expires) {
		if len(l.local) > 10000 {
			l.sweep(now)
		}
		w = &limiterWindow{expires: now.Add(window)}
		l.local[key] = w
	}
	w.count++
	return w.count
}

// Reset forgets the key, for example after a successful sign-in.
func (l *Limiter) Reset(ctx context.Context, key string) {
	if l.cache != nil {
		_ = l.cache.Del(limiterKey(key))
	}
	l.mu.Lock()
	delete(l.local, key)
	l.mu.Unlock()
}

func (l *Limiter) sweep(now time.Time) {
	for key, w := range l.local {
		if now.After(w.expires) {
			delete(l.local, key)
		}
	}
}

func limiterKey(key string) string { return "edms:limit:" + key }
