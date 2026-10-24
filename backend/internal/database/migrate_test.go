package database_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/database"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
)

const scratchTable = "migrate_runner_scratch"

func TestMigrate_AppliesPendingOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	cleanup := func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS " + scratchTable)
		_, _ = db.Exec("DELETE FROM schema_migrations WHERE version = '999_test.sql'")
	}
	cleanup()
	t.Cleanup(cleanup)

	dir := t.TempDir()
	// 001_init.sql must NOT be executed: the test DB already has `links`, so the
	// runner only records it. Make its content fail loudly if it were run.
	if err := os.WriteFile(filepath.Join(dir, "001_init.sql"),
		[]byte("-- would fail if executed\nTHIS IS NOT SQL;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "999_test.sql"), []byte(
		"-- scratch table for the runner test\n"+
			"CREATE TABLE IF NOT EXISTS "+scratchTable+" (\n"+
			"    id INT PRIMARY KEY -- inline note; not a terminator\n"+
			");\n"+
			"INSERT INTO "+scratchTable+" (id) VALUES (1);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	applied, err := database.Migrate(ctx, db, dir)
	if err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if !reflect.DeepEqual(applied, []string{"999_test.sql"}) {
		t.Fatalf("first Migrate applied %v, want [999_test.sql]", applied)
	}

	var n int
	if err := db.Get(&n, "SELECT COUNT(*) FROM "+scratchTable); err != nil || n != 1 {
		t.Fatalf("scratch table rows = %d, err = %v; want 1 row", n, err)
	}
	if err := db.Get(&n, "SELECT COUNT(*) FROM schema_migrations WHERE version IN ('001_init.sql','999_test.sql')"); err != nil || n != 2 {
		t.Fatalf("recorded versions = %d, err = %v; want 2", n, err)
	}

	applied, err = database.Migrate(ctx, db, dir)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("second Migrate applied %v, want none", applied)
	}
	if err := db.Get(&n, "SELECT COUNT(*) FROM "+scratchTable); err != nil || n != 1 {
		t.Fatalf("scratch table rows after rerun = %d, err = %v; want 1", n, err)
	}
}

func TestSplitStatements(t *testing.T) {
	in := "-- header comment\nCREATE TABLE a (\n  x INT -- note\n);\n\n-- between\nINSERT INTO a VALUES (1);\nUPDATE a SET x = 2\nWHERE x = 1;\n"
	got := database.SplitStatements(in)
	want := []string{
		"CREATE TABLE a (\n  x INT -- note\n)",
		"INSERT INTO a VALUES (1)",
		"UPDATE a SET x = 2\nWHERE x = 1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitStatements =\n%#v\nwant\n%#v", got, want)
	}
}
