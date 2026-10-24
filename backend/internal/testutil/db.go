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
		dsn = "urlshortener:password@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27"
	}
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	// Clean tables before each test, children before parents (FK order).
	for _, table := range []string{
		"page_views",
		"page_blocks",
		"pages",
		"qr_tags",
		"qr_codes",
		"dashboards",
		"clicks",
		"link_tags",
		"dynamic_routing_rules",
		"links",
		"api_tokens",
		"activity_log",
		"`groups`",
		"domains",
		"users",
	} {
		_, _ = db.Exec("DELETE FROM " + table)
	}

	return db
}
