package storage

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

type Config struct {
	APIKey   string
	APIToken string
}

func New() (*Storage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configDir := filepath.Join(home, ".config", "trecli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(configDir, "trecli.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY DEFAULT 1,
			api_key TEXT NOT NULL,
			api_token TEXT NOT NULL
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) GetConfig() (*Config, error) {
	var config Config
	err := s.db.QueryRow(`SELECT api_key, api_token FROM config WHERE id = 1`).Scan(&config.APIKey, &config.APIToken)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (s *Storage) SaveConfig(cfg *Config) error {
	_, err := s.db.Exec(`
		INSERT INTO config (id, api_key, api_token) 
		VALUES (1, ?, ?) 
		ON CONFLICT(id) DO UPDATE SET 
			api_key=excluded.api_key, 
			api_token=excluded.api_token;
	`, cfg.APIKey, cfg.APIToken)
	return err
}

func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
