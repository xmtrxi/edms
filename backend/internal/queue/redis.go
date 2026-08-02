package queue

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/hibiken/asynq"
)

func RedisOptions(addr string) (asynq.RedisClientOpt, error) {
	if addr == "" {
		return asynq.RedisClientOpt{}, fmt.Errorf("redis url is required")
	}

	u, err := url.Parse(addr)
	if err != nil {
		return asynq.RedisClientOpt{}, fmt.Errorf("parse redis url: %w", err)
	}

	opts := asynq.RedisClientOpt{
		Addr: u.Host,
	}
	if u.User != nil {
		opts.Username = u.User.Username()
		opts.Password, _ = u.User.Password()
	}
	if db := u.Query().Get("db"); db != "" {
		if n, err := strconv.Atoi(db); err == nil {
			opts.DB = n
		}
	}

	return opts, nil
}
