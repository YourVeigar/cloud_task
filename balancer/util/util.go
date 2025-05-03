package util

import (
	"log"
	"net/http"
	"net/url"
)

// IsBackendAlive проверяет доступность бэкенда по указанному URL с добавлением пути "/health"
func IsBackendAlive(backendURL url.URL) bool {
	// Формируем полный URL для health check
	checkURL := backendURL.String() + "/health"
	resp, err := http.Get(checkURL)
	if err != nil {
		// Логируем ошибку, если бэкенд недоступен
		log.Printf("[ERROR] Backend %s is down: %v", checkURL, err)
		return false
	}
	defer resp.Body.Close()

	// Проверяем код ответа от сервера
	if resp.StatusCode != http.StatusOK {
		log.Printf("[WARNING] Backend %s returned status: %d", checkURL, resp.StatusCode)
		return false
	}

	log.Printf("[INFO] Backend %s is up and running", checkURL)
	return true
}
