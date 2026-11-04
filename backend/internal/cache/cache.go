package cache

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/valkey-io/valkey-go"
)

type Cache struct {
	Client valkey.Client
}

var ctx = context.Background()

func NewClient(addr string) (*Cache, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	var (
		username string
		password string
	)
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{u.Host},
		Username:    username,
		Password:    password,
	})
	if err != nil {
		return nil, err
	}

	return &Cache{
		Client: client,
	}, nil
}

func (c *Cache) Close() error {
	c.Client.Close()
	return nil
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.Client.Do(ctx, c.Client.B().Ping().Build()).Error()
}

func (c *Cache) Set(key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Client.Do(ctx, c.Client.B().Set().Key(key).Value(string(data)).Ex(ttl).Build()).Error()
}

func (c *Cache) Get(key string) (string, error) {
	return c.Client.Do(ctx, c.Client.B().Get().Key(key).Build()).ToString()
}

func (c *Cache) SetRaw(key, value string, ttl time.Duration) error {
	return c.Client.Do(ctx, c.Client.B().Set().Key(key).Value(value).Ex(ttl).Build()).Error()
}

func GetAs[T any](c *Cache, key string) (T, error) {
	var zero T
	result, err := c.Client.Do(ctx, c.Client.B().Get().Key(key).Build()).ToString()
	if err != nil {
		return zero, err
	}
	if err := json.Unmarshal([]byte(result), &zero); err != nil {
		return zero, err
	}
	return zero, nil
}

func (c *Cache) Del(key string) error {
	return c.Client.Do(ctx, c.Client.B().Del().Key(key).Build()).Error()
}

func (c *Cache) Increment(key string) error {
	return c.Client.Do(ctx, c.Client.B().Incr().Key(key).Build()).Error()
}

// IncrementWindow increments a distributed counter and starts its expiry on
// first use. It is suitable for coarse API throttles shared by all instances.
func (c *Cache) IncrementWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	count, err := c.Client.Do(ctx, c.Client.B().Incr().Key(key).Build()).ToInt64()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := c.Client.Do(ctx, c.Client.B().Expire().Key(key).Seconds(int64(window/time.Second)).Build()).Error(); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func (c *Cache) Decrement(key string) error {
	return c.Client.Do(ctx, c.Client.B().Decr().Key(key).Build()).Error()
}

func (c *Cache) IncrementBy(key string, value int64) error {
	return c.Client.Do(ctx, c.Client.B().Incrby().Key(key).Increment(value).Build()).Error()
}

func (c *Cache) DecrementBy(key string, value int64) error {
	return c.Client.Do(ctx, c.Client.B().Decrby().Key(key).Decrement(value).Build()).Error()
}

func (c *Cache) Flush() error {
	return c.Client.Do(ctx, c.Client.B().Flushdb().Build()).Error()
}
