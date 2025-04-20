package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// Структура для хранения контента
type CrawledContent struct {
	DOMAIN string
	URL    string
	//HTML        string
	TextContent string
	Title       string
	Status      int
	Metadata    map[string]string
	ContentHash string
	CrawledAt   time.Time
}

// DatabaseConfig настройки подключения
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"ssl_mode"`
}

// ContentStorage интерфейс для работы с хранилищем
type Storage interface {
	Save(ctx context.Context, content *CrawledContent) error
	Exists(ctx context.Context, contentHash string) (bool, error)
	Close() error
}

// PostgresStorage реализация для PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(cfg DatabaseConfig) (*PostgresStorage, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %v", err)
	}

	return &PostgresStorage{db: db}, nil
}

// Init создает таблицы (вызывается при старте)
func (s *PostgresStorage) Init() error {
	query := `CREATE TABLE IF NOT EXISTS crawled_content (
		id SERIAL PRIMARY KEY,
		domain TEXT NOT NULL,
		url TEXT NOT NULL,
		text_content TEXT,
		title TEXT,
		status INT,
		metadata JSONB,
		content_hash TEXT NOT NULL UNIQUE,
		crawled_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	
	CREATE INDEX IF NOT EXISTS idx_content_hash ON crawled_content(content_hash);
	CREATE INDEX IF NOT EXISTS idx_url ON crawled_content(url);
	CREATE INDEX IF NOT EXISTS idx_crawled_at ON crawled_content(crawled_at);`

	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) Save(ctx context.Context, content *CrawledContent) error {
	metadataJSON, err := json.Marshal(content.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %v", err)
	}

	query := `INSERT INTO crawled_content (
		domain, url, text_content, title, status, metadata, content_hash, crawled_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (content_hash) DO NOTHING`

	_, err = s.db.ExecContext(ctx, query,
		content.DOMAIN,
		content.URL,
		//content.HTML,
		content.TextContent,
		content.Title,
		content.Status,
		metadataJSON,
		content.ContentHash,
		content.CrawledAt,
	)

	return err
}

func (s *PostgresStorage) ExistsByURL(ctx context.Context, url string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM crawled_content WHERE url = $1)`
	err := s.db.QueryRowContext(ctx, query, url).Scan(&exists)
	return exists, err
}

func (s *PostgresStorage) Exists(ctx context.Context, contentHash string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM crawled_content WHERE content_hash = $1)`
	err := s.db.QueryRowContext(ctx, query, contentHash).Scan(&exists)
	return exists, err
}

type StatContent struct {
	Domain string `postgres:"domain"`
	Url    string
	Status int
}

func (s *PostgresStorage) GetAll(table *[]StatContent) error {
	rows, err := s.db.Query("SELECT domain, url, status FROM crawled_content")
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		line := &StatContent{}
		if err := rows.Scan(&line.Domain, &line.Url, &line.Status); err != nil {
			log.Fatal(err)
			return err
		}
		*table = append(*table, *line)
	}

	if err = rows.Err(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
