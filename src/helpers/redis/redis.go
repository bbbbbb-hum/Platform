package redis

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	redislib "github.com/redis/go-redis/v9"
)

// Config holds Redis connection settings.
type Config struct {
	Addr        string
	Username    string
	Password    string
	DB          int
	PoolSize    int
	MinIdleConns int
	MaxRetries  int
	DialTimeout time.Duration
	ReadTimeout time.Duration
	WriteTimeout time.Duration
	PoolTimeout time.Duration
	Prefix      string
}

var (
	client   *redislib.Client
	clientMu sync.RWMutex
	keyPrefix string
)

// Init creates the global Redis client and validates the connection.
func Init(cfg Config) (*redislib.Client, error) {
	if cfg.Addr == "" {
		return nil, errors.New("redis addr is empty")
	}

	opts := &redislib.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	}

	c := redislib.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	clientMu.Lock()
	client = c
	keyPrefix = normalizePrefix(cfg.Prefix)
	clientMu.Unlock()

	return c, nil
}

// Client returns the global Redis client instance.
func Client() *redislib.Client {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return client
}

// Close closes the global Redis client.
func Close() error {
	clientMu.Lock()
	defer clientMu.Unlock()
	if client == nil {
		return nil
	}
	err := client.Close()
	client = nil
	return err
}

// BuildKey builds a namespaced Redis key with the configured prefix.
func BuildKey(parts ...string) string {
	clientMu.RLock()
	prefix := keyPrefix
	clientMu.RUnlock()

	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		cleanParts = append(cleanParts, strings.Trim(part, ":"))
	}

	if prefix == "" {
		return strings.Join(cleanParts, ":")
	}
	if len(cleanParts) == 0 {
		return prefix
	}
	return prefix + ":" + strings.Join(cleanParts, ":")
}

func normalizePrefix(prefix string) string {
	return strings.Trim(prefix, ":")
}
