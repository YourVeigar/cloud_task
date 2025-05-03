package handler

import (
	"cloud-test-task/dto"
	"cloud-test-task/rateLimiter"
	"cloud-test-task/service"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ConfigHandler отвечает за обработку HTTP-запросов для управления конфигурациями токен-бакетов
type ConfigHandler struct {
	s  service.ConfigService
	rl *rateLimiter.RateLimiter
}

// NewConfigHandler создаёт новый экземпляр обработчика конфигураций.
func NewConfigHandler(s service.ConfigService, rl *rateLimiter.RateLimiter) *ConfigHandler {
	return &ConfigHandler{s: s, rl: rl}
}

// Create обрабатывает запрос на создание нового бакета
func (h *ConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	var cfg dto.ConfigDto

	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	interval, err := time.ParseDuration(cfg.RefillInterval)
	if err != nil {
		log.Printf("[ERROR] failed to parse duration: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Failed to parse duration: " + err.Error()))
		return
	}

	bucket := rateLimiter.TokenBucketConfig{
		ClientId:       cfg.ClientId,
		Capacity:       cfg.Capacity,
		RefillInterval: interval,
	}

	if err := h.s.Create(bucket); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	if err := h.rl.AddBucket(bucket); err != nil {
		_ = h.s.Delete(bucket.ClientId) // Откат в случае ошибки
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("%+v", bucket)))
}

// GetAll возвращает список всех конфигураций бакетов
func (h *ConfigHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	configs := h.rl.GetAllBucketConfigs()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(configs); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

// GetById возвращает конфигурацию бакета по clientId
func (h *ConfigHandler) GetById(w http.ResponseWriter, r *http.Request) {
	clientId := r.URL.Path
	if clientId == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	config, err := h.rl.GetBucketConfig(clientId)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(config); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

// Update обновляет параметры конфигурации существующего бакета
func (h *ConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	var cfg dto.ConfigDto

	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	interval, err := time.ParseDuration(cfg.RefillInterval)
	if err != nil {
		log.Printf("[ERROR] failed to parse duration: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Failed to parse duration"))
		return
	}

	bucket := rateLimiter.TokenBucketConfig{
		ClientId:       cfg.ClientId,
		Capacity:       cfg.Capacity,
		RefillInterval: interval,
	}

	if err := h.s.Update(bucket); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	if err := h.rl.AddBucket(bucket); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(bucket); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

// Delete удаляет конфигурацию и соответствующий бакет по clientId
func (h *ConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientId string `json:"client_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	if err := h.rl.DeleteBucket(req.ClientId); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	if err := h.s.Delete(req.ClientId); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}
