package service

import (
	"cloud-test-task/rateLimiter"
	"cloud-test-task/repository"
)

// ConfigService описывает интерфейс для работы с конфигурациями клиентов
type ConfigService interface {
	Create(cfg rateLimiter.TokenBucketConfig) error
	GetByClientId(clientId string) (rateLimiter.TokenBucketConfig, error)
	Update(cfg rateLimiter.TokenBucketConfig) error
	Delete(clientId string) error
	GetAll() ([]rateLimiter.TokenBucketConfig, error)
}

// configService реализует интерфейс ConfigService, взаимодействуя с хранилищем
type configService struct {
	repo repository.ConfigRepo
}

// NewConfigService возвращает новый экземпляр configService
func NewConfigService(repo repository.ConfigRepo) ConfigService {
	return &configService{repo: repo}
}

// Create создает новую конфигурацию для клиента
func (s *configService) Create(cfg rateLimiter.TokenBucketConfig) error {
	return s.repo.Create(cfg)
}

// Update обновляет конфигурацию для клиента
func (s *configService) Update(cfg rateLimiter.TokenBucketConfig) error {
	return s.repo.Update(cfg)
}

// GetByClientId получает конфигурацию клиента по его ID
func (s *configService) GetByClientId(clientId string) (rateLimiter.TokenBucketConfig, error) {
	return s.repo.FindByClientID(clientId)
}

// Delete удаляет конфигурацию клиента
func (s *configService) Delete(clientId string) error {
	return s.repo.Delete(clientId)
}

// GetAll возвращает все конфигурации клиентов
func (s *configService) GetAll() ([]rateLimiter.TokenBucketConfig, error) {
	return s.repo.ListAll()
}
