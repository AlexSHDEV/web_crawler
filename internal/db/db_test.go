package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

var sqlOpen = sql.Open

func TestNewPostgresStorage(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer db.Close()

		mock.ExpectPing()

		cfg := DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Password: "postgres",
			DBName:   "postgres",
			SSLMode:  "disable",
		}

		// Replace sql.Open with a function that returns our mock db
		originalOpen := sqlOpen
		sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
			return db, nil
		}
		defer func() { sqlOpen = originalOpen }()

		_, err = NewPostgresStorage(cfg)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

}

func TestPostgresStorage_Save(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	storage := &PostgresStorage{db: db}
	ctx := context.Background()

	content := &CrawledContent{
		DOMAIN:      "example.com",
		URL:         "https://example.com",
		TextContent: "Some text content",
		Title:       "Example Title",
		Status:      200,
		Metadata:    map[string]string{},
		ContentHash: "abc123",
		CrawledAt:   time.Now(),
	}

	t.Run("successful save", func(t *testing.T) {
		metadataJSON, _ := json.Marshal(content.Metadata)

		mock.ExpectExec("INSERT INTO crawled_content").
			WithArgs(
				content.DOMAIN,
				content.URL,
				content.TextContent,
				content.Title,
				content.Status,
				metadataJSON,
				content.ContentHash,
				content.CrawledAt,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := storage.Save(ctx, content)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	storage := &PostgresStorage{db: db}
	ctx := context.Background()
	contentHash := "abc123"

	t.Run("exists", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(contentHash).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := storage.Exists(ctx, contentHash)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not exist", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(contentHash).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		exists, err := storage.Exists(ctx, contentHash)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query error", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(contentHash).
			WillReturnError(errors.New("query failed"))

		exists, err := storage.Exists(ctx, contentHash)
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "query failed")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_ExistsByURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	storage := &PostgresStorage{db: db}
	ctx := context.Background()
	url := "https://example.com"

	t.Run("exists", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(url).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := storage.ExistsByURL(ctx, url)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not exist", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(url).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		exists, err := storage.ExistsByURL(ctx, url)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query error", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(url).
			WillReturnError(errors.New("query failed"))

		exists, err := storage.ExistsByURL(ctx, url)
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "query failed")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_Close(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	storage := &PostgresStorage{db: db}

	t.Run("successful close", func(t *testing.T) {
		mock.ExpectClose()

		err := storage.Close()
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
