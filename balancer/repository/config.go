package repository

import (
	"cloud-test-task/rateLimiter"
	"database/sql"
	"log"
)

// ConfigRepo описывает контракт для взаимодействия с хранилищем конфигураций
type ConfigRepo interface {
	Create(cfg rateLimiter.TokenBucketConfig) error
	FindByClientID(clientId string) (rateLimiter.TokenBucketConfig, error)
	Update(cfg rateLimiter.TokenBucketConfig) error
	Delete(clientId string) error
	ListAll() ([]rateLimiter.TokenBucketConfig, error)
}

// configRepo реализует интерфейс ConfigRepo с использованием SQL
type configRepo struct {
	db *sql.DB
}

// NewConfigRepo возвращает реализацию интерфейса ConfigRepo
func NewConfigRepo(db *sql.DB) ConfigRepo {
	return &configRepo{db: db}
}

// Create вставляет новую конфигурацию клиента
func (r *configRepo) Create(cfg rateLimiter.TokenBucketConfig) error {
	query := `INSERT INTO client_configs (client_id, capacity, refill_interval) VALUES ($1, $2, $3)`
	if _, err := r.db.Exec(query, cfg.ClientId, cfg.Capacity, cfg.RefillInterval); err != nil {
		log.Printf("[ERROR] failed to insert config: %v", err)
		return err
	}
	return nil
}

// FindByClientID получает конфигурацию по ID клиента
func (r *configRepo) FindByClientID(clientId string) (rateLimiter.TokenBucketConfig, error) {
	var cfg rateLimiter.TokenBucketConfig
	query := `SELECT client_id, capacity, refill_interval FROM client_configs WHERE client_id = $1`
	if err := r.db.QueryRow(query, clientId).Scan(&cfg.ClientId, &cfg.Capacity, &cfg.RefillInterval); err != nil {
		log.Printf("[ERROR] failed to fetch config for %s: %v", clientId, err)
		return cfg, err
	}
	return cfg, nil
}

// Update обновляет существующую конфигурацию клиента
func (r *configRepo) Update(cfg rateLimiter.TokenBucketConfig) error {
	query := `UPDATE client_configs SET capacity = $1, refill_interval = $2 WHERE client_id = $3`
	if _, err := r.db.Exec(query, cfg.Capacity, cfg.RefillInterval, cfg.ClientId); err != nil {
		log.Printf("[ERROR] failed to update config: %v", err)
		return err
	}
	return nil
}

// Delete удаляет конфигурацию по ID клиента
func (r *configRepo) Delete(clientId string) error {
	query := `DELETE FROM client_configs WHERE client_id = $1`
	if _, err := r.db.Exec(query, clientId); err != nil {
		log.Printf("[ERROR] failed to delete config for %s: %v", clientId, err)
		return err
	}
	return nil
}

// ListAll возвращает все конфигурации клиентов
func (r *configRepo) ListAll() ([]rateLimiter.TokenBucketConfig, error) {
	query := `SELECT client_id, capacity, refill_interval FROM client_configs`
	rows, err := r.db.Query(query)
	if err != nil {
		log.Printf("[ERROR] failed to list configs: %v", err)
		return nil, err
	}
	defer rows.Close()

	var result []rateLimiter.TokenBucketConfig
	for rows.Next() {
		var cfg rateLimiter.TokenBucketConfig
		if err := rows.Scan(&cfg.ClientId, &cfg.Capacity, &cfg.RefillInterval); err != nil {
			log.Printf("[ERROR] failed to scan config row: %v", err)
			return nil, err
		}
		result = append(result, cfg)
	}
	return result, nil
}
