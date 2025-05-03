package db

import (
	"database/sql"
	"fmt"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/stdlib"
)

// NewDB инициализирует подключение к PostgreSQL и создаёт таблицу client_configs, если она отсутствует
func NewDB(uri string) (*sql.DB, error) {
	// Парсим URI подключения в конфигурацию pgx
	connCfg, err := pgx.ParseURI(uri)
	if err != nil {
		return nil, fmt.Errorf("pgx parse config error: %v", err)
	}

	// Открываем стандартное подключение к базе данных через pgx
	db := stdlib.OpenDB(connCfg)

	// Проверяем соединение с базой данных
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Создаём таблицу client_configs, если она ещё не существует
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS client_configs (
			client_id TEXT PRIMARY KEY,
			capacity INTEGER NOT NULL,
			refill_interval BIGINT NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create client_configs table: %v", err)
	}

	return db, nil
}
