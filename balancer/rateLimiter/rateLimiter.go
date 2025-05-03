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
	TokenBucketConfig

	tokens int
	mu     sync.Mutex
}

// NewTokenBucket создаёт новый бакет с заданной вместимостью и интервалом пополнения
func NewTokenBucket(cfg TokenBucketConfig) *TokenBucket {
	return &TokenBucket{
		TokenBucketConfig: cfg,
		tokens:            cfg.Capacity,
	}
}

// Refill запускает процесс периодического пополнения токенов
func (tb *TokenBucket) Refill() {
	ticker := time.NewTicker(tb.RefillInterval)
	for {
		tb.mu.Lock()
		tb.tokens = tb.Capacity
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
	buckets map[string]*TokenBucket
	db      *sql.DB
	mu      sync.RWMutex
}

// NewRateLimiter инициализирует ограничитель запросов и загружает конфиги
func NewRateLimiter(db *sql.DB) (*RateLimiter, error) {
	rl := &RateLimiter{
		buckets: make(map[string]*TokenBucket),
		db:      db,
	}

	log.Print("Loading default configs")
	if err := rl.loadDefaultConfig(); err != nil {
		return nil, fmt.Errorf("failed to load default configs: %w", err)
	}
	log.Print("Loading client configs from db")
	if err := rl.loadClientConfig(); err != nil {
		log.Print("[ERROR] Failed to load configs from db: ", err)
	}

	for _, bucket := range rl.buckets {
		go bucket.Refill()
	}

	return rl, nil
}

// AddBucket добавляет или обновляет бакет клиента
func (rl *RateLimiter) AddBucket(cfg TokenBucketConfig) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.buckets[cfg.ClientId] = NewTokenBucket(cfg)

	go rl.buckets[cfg.ClientId].Refill()

	log.Printf("[DEBUG] added/updated bucket %s: %+v", cfg.ClientId, cfg)

	return nil
}

// DeleteBucket удаляет бакет и конфиг клиента
func (rl *RateLimiter) DeleteBucket(clientId string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	_, bucketExists := rl.buckets[clientId]
	if !bucketExists {
		return fmt.Errorf("backend %s does not exist", clientId)
	}

	delete(rl.buckets, clientId)
	return nil
}

// GetBucketConfig возвращает конфигурацию бакета по ID клиента
func (rl *RateLimiter) GetBucketConfig(clientId string) (TokenBucketConfig, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	cfg, exists := rl.buckets[clientId]
	if !exists {
		return TokenBucketConfig{}, fmt.Errorf("backend %s does not exist", clientId)
	}
	return cfg.TokenBucketConfig, nil
}

// GetAllBucketConfigs возвращает список всех конфигураций
func (rl *RateLimiter) GetAllBucketConfigs() []TokenBucketConfig {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	all := make([]TokenBucketConfig, 0, len(rl.buckets))
	for _, cfg := range rl.buckets {
		all = append(all, cfg.TokenBucketConfig)
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

	defaultCap := viper.GetInt("rate-limiter.capacity")
	defaultInterval := viper.GetDuration("rate-limiter.refill-interval")

	clients := viper.GetStringSlice("backend-servers")

	for _, rawUrl := range clients {
		u, err := url.Parse(rawUrl)
		if err != nil {
			return fmt.Errorf("parse client id failed: %v", err)
		}

		clientId := u.Host
		rl.buckets[clientId] = NewTokenBucket(TokenBucketConfig{
			ClientId:       clientId,
			Capacity:       defaultCap,
			RefillInterval: defaultInterval,
		})
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

		rl.buckets[cfg.ClientId] = NewTokenBucket(cfg)
	}

	return rows.Err()
}

// Allow проверяет и при необходимости создаёт бакет, затем вызывает Allow на нём
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[clientID]
	rl.mu.RUnlock()

	if !exists {
		log.Panicf("config for client %s not exists", clientID)
	}

	return bucket.Allow()
}
