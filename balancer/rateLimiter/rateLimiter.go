package rateLimiter

import (
	"database/sql"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"net/url"
	"sync"
	"time"
)

// TokenBucket реализует механизм токен-бакета для ограничения количества запросов
type TokenBucket struct {
	capacity       int
	tokens         int
	refillInterval time.Duration
	mu             sync.Mutex
}

// NewTokenBucket создаёт новый бакет с заданной вместимостью и интервалом пополнения
func NewTokenBucket(cap int, interval time.Duration) *TokenBucket {
	return &TokenBucket{
		capacity:       cap,
		tokens:         cap,
		refillInterval: interval,
	}
}

// Refill запускает процесс периодического пополнения токенов
func (tb *TokenBucket) Refill() {
	ticker := time.NewTicker(tb.refillInterval)
	for {
		tb.mu.Lock()
		tb.tokens = tb.capacity
		tb.mu.Unlock()
		<-ticker.C
	}
}

// Allow проверяет наличие токена и, если он есть, уменьшает их количество
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// TokenBucketConfig описывает параметры бакета конкретного клиента
type TokenBucketConfig struct {
	ClientId       string        `json:"client_id"`
	Capacity       int           `json:"capacity"`
	RefillInterval time.Duration `json:"refill_interval"`
}

// RateLimiter управляет бакетами клиентов и их конфигурациями
type RateLimiter struct {
	configs map[string]*TokenBucketConfig
	buckets map[string]*TokenBucket
	db      *sql.DB
	mu      sync.RWMutex
}

// NewRateLimiter инициализирует ограничитель запросов и загружает конфиги
func NewRateLimiter(db *sql.DB) *RateLimiter {
	rl := &RateLimiter{
		configs: make(map[string]*TokenBucketConfig),
		buckets: make(map[string]*TokenBucket),
		db:      db,
	}

	log.Print("loading default config")
	if err := rl.loadDefaultConfig(); err != nil {
		log.Fatal("load bucket config failed", err)
		return nil
	}
	if err := rl.loadClientConfig(); err != nil {
		log.Print("[ERROR] load bucket config from db failed", err)
	}
	return rl
}

// AddBucket добавляет или обновляет бакет клиента
func (rl *RateLimiter) AddBucket(cfg TokenBucketConfig) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.buckets[cfg.ClientId] = NewTokenBucket(cfg.Capacity, cfg.RefillInterval)
	rl.configs[cfg.ClientId] = &cfg

	log.Printf("[DEBUG] added/updated bucket %s: %+v", cfg.ClientId, cfg)
	return nil
}

// DeleteBucket удаляет бакет и конфиг клиента
func (rl *RateLimiter) DeleteBucket(clientId string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	_, bucketExists := rl.buckets[clientId]
	_, configExists := rl.configs[clientId]

	if !bucketExists && !configExists {
		return fmt.Errorf("backend %s does not exist", clientId)
	}

	if bucketExists {
		delete(rl.buckets, clientId)
	}
	if configExists {
		delete(rl.configs, clientId)
	}
	return nil
}

// GetBucketConfig возвращает конфигурацию бакета по ID клиента
func (rl *RateLimiter) GetBucketConfig(clientId string) (*TokenBucketConfig, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	cfg, exists := rl.configs[clientId]
	if !exists {
		return nil, fmt.Errorf("backend %s does not exist", clientId)
	}
	return cfg, nil
}

// GetAllBucketConfigs возвращает список всех конфигураций
func (rl *RateLimiter) GetAllBucketConfigs() []TokenBucketConfig {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	var all []TokenBucketConfig
	for _, cfg := range rl.configs {
		all = append(all, *cfg)
	}
	return all
}

// loadDefaultConfig загружает дефолтные значения конфигурации из файла
func (rl *RateLimiter) loadDefaultConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	defaultCap := viper.GetInt("rate_limiting.capacity")
	defaultInterval := viper.GetDuration("rate_limiting.refill_interval")

	clients := viper.GetStringSlice("backends")

	for _, rawUrl := range clients {
		u, err := url.Parse(rawUrl)
		if err != nil {
			return fmt.Errorf("parse client id failed: %v", err)
		}

		clientId := u.Host
		rl.configs[clientId] = &TokenBucketConfig{
			ClientId:       clientId,
			Capacity:       defaultCap,
			RefillInterval: defaultInterval,
		}
	}
	return nil
}

// loadClientConfig загружает пользовательские настройки из БД
func (rl *RateLimiter) loadClientConfig() error {
	rows, err := rl.db.Query("SELECT client_id, capacity, refill_interval FROM client_configs")
	if err != nil {
		log.Printf("Failed to load client configs: %v", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cfg TokenBucketConfig
		var intervalSec int

		if err := rows.Scan(&cfg.ClientId, &cfg.Capacity, &intervalSec); err != nil {
			log.Printf("Failed to scan client config: %v", err)
			continue
		}
		cfg.RefillInterval = time.Duration(intervalSec) * time.Second

		rl.configs[cfg.ClientId] = &cfg
		rl.buckets[cfg.ClientId] = NewTokenBucket(cfg.Capacity, cfg.RefillInterval)
	}

	return rows.Err()
}

// Allow проверяет и при необходимости создаёт бакет, затем вызывает Allow на нём
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[clientID]
	cfg, hasConfig := rl.configs[clientID]
	rl.mu.RUnlock()

	if !exists {
		if !hasConfig {
			log.Fatalf("config for client %s not exists", clientID)
		}
		rl.mu.Lock()
		bucket = NewTokenBucket(cfg.Capacity, cfg.RefillInterval)
		rl.buckets[clientID] = bucket
		go bucket.Refill()
		rl.mu.Unlock()
	}

	return bucket.Allow()
}
