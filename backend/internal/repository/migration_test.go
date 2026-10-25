package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/database"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
)

// TestMigration002_Backfill assumes 002_bitly_clone.sql has already been
// applied to the test database. It inserts a pre-migration style user and
// link (no group), re-runs the idempotent migration file and checks that the
// backfill created exactly one group and attached the user and link to it.
func TestMigration002_Backfill(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Use a single connection so FOREIGN_KEY_CHECKS=0 applies to the inserts:
	// a link created before migration 002 has group_id=0 (the column default).
	conn, err := db.Connx(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		t.Fatalf("disable fk checks: %v", err)
	}
	res, err := conn.ExecContext(ctx,
		"INSERT INTO users (email, password_hash) VALUES ('backfill.user@example.com', 'x')")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()
	res, err = conn.ExecContext(ctx,
		"INSERT INTO links (user_id, short_code, original_url) VALUES (?, 'bkfill1', 'https://example.com/a')", userID)
	if err != nil {
		t.Fatalf("insert link: %v", err)
	}
	linkID, _ := res.LastInsertId()
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1"); err != nil {
		t.Fatalf("enable fk checks: %v", err)
	}
	_ = conn.Close()

	raw, err := os.ReadFile("../../migrations/002_bitly_clone.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	// Run twice: the file must be idempotent and must not duplicate groups.
	for run := 1; run <= 2; run++ {
		for _, stmt := range database.SplitStatements(string(raw)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("run %d: exec %q: %v", run, stmt, err)
			}
		}
	}

	var groups int
	if err := db.GetContext(ctx, &groups, "SELECT COUNT(*) FROM `groups` WHERE user_id = ?", userID); err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if groups != 1 {
		t.Fatalf("expected 1 group for user, got %d", groups)
	}

	var g struct {
		ID   uint64 `db:"id"`
		Name string `db:"name"`
	}
	if err := db.GetContext(ctx, &g, "SELECT id, name FROM `groups` WHERE user_id = ?", userID); err != nil {
		t.Fatalf("get group: %v", err)
	}
	if g.Name != "backfill.user" {
		t.Errorf("group name = %q, want %q", g.Name, "backfill.user")
	}

	var defaultGroup *uint64
	if err := db.GetContext(ctx, &defaultGroup, "SELECT default_group_id FROM users WHERE id = ?", userID); err != nil {
		t.Fatalf("get default group: %v", err)
	}
	if defaultGroup == nil || *defaultGroup != g.ID {
		t.Errorf("users.default_group_id = %v, want %d", defaultGroup, g.ID)
	}

	var link struct {
		GroupID  uint64 `db:"group_id"`
		Archived bool   `db:"archived"`
		Source   string `db:"source"`
	}
	if err := db.GetContext(ctx, &link, "SELECT group_id, archived, source FROM links WHERE id = ?", linkID); err != nil {
		t.Fatalf("select new link columns: %v", err)
	}
	if link.GroupID != g.ID {
		t.Errorf("links.group_id = %d, want %d", link.GroupID, g.ID)
	}
	if link.Archived {
		t.Error("expected archived = false")
	}
	if link.Source != "link" {
		t.Errorf("source = %q, want link", link.Source)
	}
}

// TestMigration002_DomainDefaults checks that the domain columns exist
// and that a domain created after the migration defaults to 'pending'.
func TestMigration002_DomainDefaults(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	res, err := db.ExecContext(ctx, "INSERT INTO users (email, password_hash) VALUES ('dom@example.com', 'x')")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx, "INSERT INTO domains (user_id, domain) VALUES (?, 'new.example.com')", userID)
	if err != nil {
		t.Fatalf("insert domain: %v", err)
	}
	domainID, _ := res.LastInsertId()

	var status string
	if err := db.GetContext(ctx, &status, "SELECT verification_status FROM domains WHERE id = ?", domainID); err != nil {
		t.Fatalf("select status: %v", err)
	}
	if status != "pending" {
		t.Errorf("new domain verification_status = %q, want pending", status)
	}
}

func TestMigration003_ShortCodeCaseSensitive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	var col struct {
		ColumnType string `db:"COLUMN_TYPE"`
		Collation  string `db:"COLLATION_NAME"`
	}
	err := db.Get(&col, `SELECT COLUMN_TYPE, COLLATION_NAME FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'links' AND COLUMN_NAME = 'short_code'`)
	if err != nil {
		t.Fatalf("query column: %v", err)
	}
	if col.ColumnType != "varchar(255)" || col.Collation != "utf8mb4_bin" {
		t.Fatalf("short_code: got %s %s, want varchar(255) utf8mb4_bin", col.ColumnType, col.Collation)
	}
}

// TestMigration004_DomainKeyUnique assumes 004_links_domain_key.sql has been
// applied. It re-runs the idempotent file, then checks that two
// default-domain links (domain_id NULL) can no longer share a short code
// while a custom domain still has its own namespace.
func TestMigration004_DomainKeyUnique(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	raw, err := os.ReadFile("../../migrations/004_links_domain_key.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, stmt := range database.SplitStatements(string(raw)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("re-run %q: %v", stmt, err)
		}
	}

	res, err := db.ExecContext(ctx, "INSERT INTO users (email, password_hash) VALUES ('mig004@example.com', 'x')")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx, "INSERT INTO `groups` (user_id, name) VALUES (?, 'g')", userID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	groupID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx, "INSERT INTO domains (user_id, domain) VALUES (?, 'mig004.example.com')", userID)
	if err != nil {
		t.Fatalf("insert domain: %v", err)
	}
	domainID, _ := res.LastInsertId()

	insert := func(domain any) error {
		_, err := db.ExecContext(ctx,
			"INSERT INTO links (user_id, group_id, domain_id, short_code, original_url) VALUES (?, ?, ?, 'same', 'https://example.com')",
			userID, groupID, domain)
		return err
	}
	if err := insert(nil); err != nil {
		t.Fatalf("first default-domain link: %v", err)
	}
	if err := insert(nil); err == nil {
		t.Fatal("second default-domain link with the same code was accepted")
	}
	if err := insert(domainID); err != nil {
		t.Fatalf("same code on a custom domain: %v", err)
	}

	var oldIndex int
	if err := db.GetContext(ctx, &oldIndex, `SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'links' AND INDEX_NAME = 'idx_domain_short_code'`); err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	if oldIndex != 0 {
		t.Error("idx_domain_short_code should have been dropped")
	}
}

// TestMigration005_QRTables assumes 005_qr_codes.sql has been applied. It
// re-runs the idempotent file, checks that qr_tags accepts a row for an
// existing QR code and that deleting the owned link cascades to qr_codes and
// qr_tags.
func TestMigration005_QRTables(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	raw, err := os.ReadFile("../../migrations/005_qr_codes.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, stmt := range database.SplitStatements(string(raw)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("re-run %q: %v", stmt, err)
		}
	}

	res, err := db.ExecContext(ctx, "INSERT INTO users (email, password_hash) VALUES ('mig005@example.com', 'x')")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx, "INSERT INTO `groups` (user_id, name) VALUES (?, 'g')", userID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	groupID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		"INSERT INTO links (user_id, group_id, short_code, original_url, source) VALUES (?, ?, 'qr005', 'https://example.com', 'qr')",
		userID, groupID)
	if err != nil {
		t.Fatalf("insert link: %v", err)
	}
	linkID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx,
		"INSERT INTO qr_codes (user_id, group_id, link_id, render_customizations) VALUES (?, ?, ?, '{}')",
		userID, groupID, linkID)
	if err != nil {
		t.Fatalf("insert qr code: %v", err)
	}
	qrID, _ := res.LastInsertId()

	if _, err := db.ExecContext(ctx, "INSERT INTO qr_tags (qr_code_id, tag) VALUES (?, 'promo')", qrID); err != nil {
		t.Fatalf("insert qr tag: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO qr_codes (user_id, group_id, link_id, render_customizations) VALUES (?, ?, ?, '{}')",
		userID, groupID, linkID); err == nil {
		t.Error("second QR code for the same link was accepted")
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM links WHERE id = ?", linkID); err != nil {
		t.Fatalf("delete link: %v", err)
	}
	var qrCount, tagCount int
	if err := db.GetContext(ctx, &qrCount, "SELECT COUNT(*) FROM qr_codes"); err != nil {
		t.Fatalf("count qr codes: %v", err)
	}
	if qrCount != 0 {
		t.Errorf("qr_codes rows after link delete = %d, want 0", qrCount)
	}
	if err := db.GetContext(ctx, &tagCount, "SELECT COUNT(*) FROM qr_tags"); err != nil {
		t.Fatalf("count qr tags: %v", err)
	}
	if tagCount != 0 {
		t.Errorf("qr_tags rows after link delete = %d, want 0", tagCount)
	}
}

// TestMigration009_Idempotent re-runs 009 (already applied to the test
// database) and checks it splits into its seven CREATE TABLE statements.
func TestMigration009_Idempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	raw, err := os.ReadFile("../../migrations/009_campaigns_webhooks_bulk.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	stmts := database.SplitStatements(string(raw))
	if len(stmts) != 7 {
		t.Fatalf("got %d statements, want 7", len(stmts))
	}
	for run := 1; run <= 2; run++ {
		for _, stmt := range stmts {
			if _, err := db.ExecContext(context.Background(), stmt); err != nil {
				t.Fatalf("run %d: exec %q: %v", run, stmt, err)
			}
		}
	}
}
