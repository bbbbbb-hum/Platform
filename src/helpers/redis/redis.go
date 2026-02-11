package redis

import (
	"AgentEarth_AgentPlatform/src/helpers/config"
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	redislib "github.com/redis/go-redis/v9"
)

// Config holds Redis connection settings.
type Config struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
	Prefix       string
}

var (
	client    *redislib.Client
	clientMu  sync.RWMutex
	keyPrefix string
)

// Init creates the global Redis client and validates the connection.
func Init(cfg Config) (*redislib.Client, error) {
	if cfg.Addr == "" {
		return nil, errors.New("redis addr is empty")
	}

	opts := &redislib.Options{
		Addr:         cfg.Addr,
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

// GetString returns the value of key as a string.
// ok=true if key exists, ok=false if key does not exist.
// err is non-nil only for real errors (connection failure, etc.).
// Call BuildKey(...) beforehand if you want automatic prefixing.
func GetString(ctx context.Context, key string) (value string, ok bool, err error) {
	if key == "" {
		return "", false, errors.New("redis key is empty")
	}
	c := Client()
	if c == nil {
		return "", false, errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	value, err = c.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redislib.Nil) {
			return "", false, nil // key not exists, not a real error
		}
		return "", false, err
	}
	return value, true, nil
}

// IncrFloat adds delta to the value of key (creates key with 0 if not exists).
// newKeyTTL is only applied when the key is newly created; existing keys keep their TTL.
// Call BuildKey(...) beforehand if you want automatic prefixing.
func IncrFloat(ctx context.Context, key string, delta float64, newKeyTTL time.Duration) error {
	if key == "" {
		return errors.New("redis key is empty")
	}
	c := Client()
	if c == nil {
		return errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// GET current -> add delta -> SET -> PEXPIRE only if key was new
	script := redislib.NewScript(`
		local current = redis.call('GET', KEYS[1])
		local add = tonumber(ARGV[1])
		local ttlMs = tonumber(ARGV[2])
		local newVal = (current and tonumber(current) or 0) + add
		redis.call('SET', KEYS[1], tostring(newVal))
		if not current then
			redis.call('PEXPIRE', KEYS[1], ttlMs)
		end
		return tostring(newVal)
	`)
	ttlMs := newKeyTTL.Milliseconds()
	return script.Run(ctx, c, []string{key}, delta, ttlMs).Err()
}

// SetString sets the value of key with an optional TTL (0 means no expiration).
// Call BuildKey(...) beforehand if you want automatic prefixing.
func SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	if key == "" {
		return errors.New("redis key is empty")
	}
	c := Client()
	if c == nil {
		return errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.Set(ctx, key, value, ttl).Err()
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
	return config.GetString("server.namespace") + "_" + strings.Trim(prefix, ":")
}
