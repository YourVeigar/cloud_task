package loadbalancer

import (
	"cloud-test-task/rateLimiter"
	"cloud-test-task/util"
	"log"
	"net/url"
	"sync"
	"time"
)

// Backend содержит данные об сервере
type Backend struct {
	Url   url.URL // адрес сервера
	Alive bool    // статус доступности
}

// LoadBalancer управляет выбором и мониторингом серверов
type LoadBalancer struct {
	Backends    []*Backend
	Index       uint32
	RateLimiter *rateLimiter.RateLimiter
	mu          sync.Mutex
}

// NextBackend ищет первый доступный backend с разрешённым токеном, возвращает флаг, если ни у одного клиента нет токенов
func (lb *LoadBalancer) NextBackend() (*Backend, bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	noTokens := false
	total := len(lb.Backends)

	for i := 0; i < total; i++ {
		idx := lb.Index % uint32(total)
		lb.Index++

		current := lb.Backends[idx]
		if !current.Alive {
			continue
		}

		clientKey := current.Url.Host
		if lb.RateLimiter.Allow(clientKey) {
			return current, false
		}
		noTokens = true
	}

	return nil, noTokens
}

// HealthCheck проверяет доступность backend-серверов каждые 10 секунд.
// Если вдруг сервер лёг, то он перепроверится через 10 сек и если он работает, то снова станет доступным
func (lb *LoadBalancer) HealthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		lb.mu.Lock()
		for _, backend := range lb.Backends {
			if util.IsBackendAlive(backend.Url) {
				if !backend.Alive {
					log.Printf("Backend recovered: %v", backend)
				}
				backend.Alive = true
			} else {
				if backend.Alive {
					log.Printf("Backend went down: %v", backend)
				}
				backend.Alive = false
			}
		}
		lb.mu.Unlock()
	}
}
