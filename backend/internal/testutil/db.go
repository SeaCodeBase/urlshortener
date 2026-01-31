package testutil

import (
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func SetupTestDB(t *testing.T) *sqlx.DB {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "urlshortener:password@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC"
	}
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	// Clean tables before each test
	_, _ = db.Exec("DELETE FROM clicks")
	_, _ = db.Exec("DELETE FROM links")
	_, _ = db.Exec("DELETE FROM users")

	return db
}
