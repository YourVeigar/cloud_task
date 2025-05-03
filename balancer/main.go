package main

import (
	"cloud-test-task/config"
	"cloud-test-task/db"
	"cloud-test-task/handler"
	"cloud-test-task/loadBalancer"
	"cloud-test-task/rateLimiter"
	"cloud-test-task/repository"
	"cloud-test-task/service"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// NewReverseProxy основная функция проксирования запросов
// если в бакете для текущего клиента есть токены, то запросы передаются в него, если нет, то передаются на следующий сервер
func NewReverseProxy(lb *loadbalancer.LoadBalancer) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			backend, tokensEmpty := lb.NextBackend()
			if backend == nil {
				if tokensEmpty {
					req.URL.Host = "rate-limited"
					return
				}
				req.URL.Host = ""
				return
			}
			req.URL.Scheme = backend.Url.Scheme
			req.URL.Host = backend.Url.Host
			log.Printf("Forwarding request to %v", backend.Url)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.URL.Host == "" {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte("Backends are not available"))
			} else if len(r.URL.Host) > 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("Rate limit exceeded"))
				log.Printf("[INFO] all buckets are empty")
				return
			} else {
				w.WriteHeader(r.Response.StatusCode)
				_, err := io.Copy(w, r.Response.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
		},
	}
}

func main() {
	// Загрузка конфигурации
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Подключение к базе данных
	db, err := db.NewDB(config.Db)
	if err != nil {
		log.Fatal("failed to connect to db: ", err)
	}

	// Заполнение массива бэкендов из конфигурации
	backends := make([]*loadbalancer.Backend, 0)
	for _, backendURL := range config.Backends {
		temp, err := url.Parse(backendURL)
		if err != nil {
			log.Fatalf("Failed to parse backend URL from config: %v", err)
		}
		backends = append(backends, &loadbalancer.Backend{Url: *temp, Alive: false})
	}

	// Инициализация RateLimiter
	rl := rateLimiter.NewRateLimiter(db)

	// Инициализация LoadBalancer
	lb := &loadbalancer.LoadBalancer{
		Backends:    backends,
		Index:       0,
		RateLimiter: rl,
	}

	// Запуск проверки доступности бэкендов
	go lb.HealthCheck()

	// Создание сервера
	mux := http.NewServeMux()

	// Создание прокси для обработки запросов
	proxy := NewReverseProxy(lb)
	mux.Handle("/", proxy)

	// Инициализация репозитория и сервиса для обработки CRUD операций с лимитами
	repo := repository.NewConfigRepo(db)
	service := service.NewConfigService(repo)
	handler := handler.NewConfigHandler(service, rl)

	// CRUD эндпоинты для работы с лимитами
	mux.HandleFunc("/rate-limits", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetAll(w, r)
		case http.MethodPost:
			handler.Create(w, r)
		case http.MethodPut:
			handler.Update(w, r)
		case http.MethodDelete:
			handler.Delete(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Эндпоинт для получения лимита по ID
	mux.Handle("/rate-limits/", http.StripPrefix("/rate-limits/", http.HandlerFunc(handler.GetById)))

	// Настройка HTTP сервера
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.Port),
		Handler: mux,
	}

	log.Printf("Load balancer started on :%s", config.Port)

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, os.Kill)
	go func() {
		<-stop
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal("Server shutdown error:", err)
		}
	}()

	// Запуск HTTP сервера
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("Server error:", err)
	}
}
