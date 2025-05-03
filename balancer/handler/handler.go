package handler

import (
	"cloud-test-task/model"
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
	rateLimiter   *rateLimiter.RateLimiter
	configService service.ConfigService
}

// NewConfigHandler создаёт новый экземпляр обработчика конфигураций.
func NewConfigHandler(configService service.ConfigService, rateLimiter *rateLimiter.RateLimiter) *ConfigHandler {
	return &ConfigHandler{configService: configService, rateLimiter: rateLimiter}
}

// Create обрабатывает запрос на создание нового бакета
func (h *ConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	var cfg model.UpdateConfigRequest

	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	interval, err := time.ParseDuration(cfg.RefillInterval)
	if err != nil {
		log.Printf("[ERROR] failed to parse duration: %v", err)
		http.Error(w, "Failed to parse duration: "+err.Error(), http.StatusBadRequest)
		return
	}

	bucket := rateLimiter.TokenBucketConfig{
		ClientId:       cfg.ClientId,
		Capacity:       cfg.Capacity,
		RefillInterval: interval,
	}

	if err := h.configService.Create(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.rateLimiter.AddBucket(bucket); err != nil {
		_ = h.configService.Delete(bucket.ClientId) // Откат в случае ошибки
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(fmt.Sprintf("%+v", bucket)))
}

// GetAll возвращает список всех конфигураций бакетов
func (h *ConfigHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	configs := h.rateLimiter.GetAllBucketConfigs()

	if err := json.NewEncoder(w).Encode(configs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// GetById возвращает конфигурацию бакета по clientId
func (h *ConfigHandler) GetById(w http.ResponseWriter, r *http.Request) {
	clientId := r.URL.Path
	if clientId == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	config, err := h.rateLimiter.GetBucketConfig(clientId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// Update обновляет параметры конфигурации существующего бакета
func (h *ConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	var cfg model.UpdateConfigRequest

	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	interval, err := time.ParseDuration(cfg.RefillInterval)
	if err != nil {
		log.Printf("[ERROR] failed to parse duration: %v", err)
		http.Error(w, fmt.Sprintf("Failed to parse duration: %v", err.Error()), http.StatusBadRequest)
		return
	}

	bucket := rateLimiter.TokenBucketConfig{
		ClientId:       cfg.ClientId,
		Capacity:       cfg.Capacity,
		RefillInterval: interval,
	}

	if err := h.configService.Update(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.rateLimiter.AddBucket(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// Delete удаляет конфигурацию и соответствующий бакет по clientId
func (h *ConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var req model.DeleteConfigRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.rateLimiter.DeleteBucket(req.ClientId); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.configService.Delete(req.ClientId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
