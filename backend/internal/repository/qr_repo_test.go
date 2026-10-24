package repository_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createQR attaches a QR code to link.
func createQR(t *testing.T, db *sqlx.DB, link *model.Link, title string) *model.QRCode {
	t.Helper()
	qr := &model.QRCode{UserID: link.UserID, GroupID: link.GroupID, LinkID: link.ID,
		RenderCustomizations: model.JSONMap{"color": "#000000"}}
	if title != "" {
		qr.Title = &title
	}
	require.NoError(t, repository.NewQRRepository(db).Create(context.Background(), qr))
	return qr
}

func qrIDsOf(rows []repository.QRCodeRow) []uint64 {
	out := make([]uint64, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func TestQRRepository_CreateGetDelete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	linkTitle := "Menu"
	_, links := linkFixture(t, db, "qrcrud@example.com",
		&model.Link{ShortCode: "qrc1", Source: model.LinkSourceQR, Title: &linkTitle})
	repo := repository.NewQRRepository(db)

	qr := createQR(t, db, links[0], "Table card")
	require.NotZero(t, qr.ID)
	assert.False(t, qr.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, qr.ID)
	require.NoError(t, err)
	assert.Equal(t, links[0].ID, got.LinkID)
	require.NotNil(t, got.Title)
	assert.Equal(t, "Table card", *got.Title)
	assert.Equal(t, "#000000", got.RenderCustomizations["color"])
	assert.Equal(t, "qrc1", got.LinkShortCode)
	assert.Equal(t, "https://example.com/qrc1", got.LinkOriginalURL)
	assert.Equal(t, model.LinkSourceQR, got.LinkSource)
	require.NotNil(t, got.LinkTitle)
	assert.Equal(t, "Menu", *got.LinkTitle)
	assert.Equal(t, int64(0), got.TotalScans)

	l := got.ToLink()
	assert.Equal(t, links[0].ID, l.ID)
	assert.Equal(t, "qrc1", l.ShortCode)
	assert.Equal(t, model.LinkSourceQR, l.Source)
	assert.Equal(t, qr.GroupID, l.GroupID)

	byLink, err := repo.GetByLinkID(ctx, links[0].ID)
	require.NoError(t, err)
	assert.Equal(t, qr.ID, byLink.ID)
	_, err = repo.GetByLinkID(ctx, links[0].ID+1000)
	assert.ErrorIs(t, err, repository.ErrQRCodeNotFound)

	rows, err := repo.GetByIDs(ctx, []uint64{qr.ID, qr.ID + 1000})
	require.NoError(t, err)
	assert.Equal(t, []uint64{qr.ID}, qrIDsOf(rows))
	rows, err = repo.GetByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)

	newTitle := "Window sticker"
	qr.Title = &newTitle
	qr.RenderCustomizations = model.JSONMap{"color": "#ff0000"}
	require.NoError(t, repo.Update(ctx, qr))
	require.NoError(t, repo.SetArchived(ctx, []uint64{qr.ID}, true))
	got, err = repo.GetByID(ctx, qr.ID)
	require.NoError(t, err)
	assert.Equal(t, "Window sticker", *got.Title)
	assert.Equal(t, "#ff0000", got.RenderCustomizations["color"])
	assert.True(t, got.Archived)

	require.NoError(t, repo.Delete(ctx, qr.ID))
	_, err = repo.GetByID(ctx, qr.ID)
	assert.ErrorIs(t, err, repository.ErrQRCodeNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, qr.ID), repository.ErrQRCodeNotFound)
	assert.ErrorIs(t, repo.Update(ctx, qr), repository.ErrQRCodeNotFound)
}

func TestQRRepository_Create_DuplicateLink(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	_, links := linkFixture(t, db, "qrdup@example.com", &model.Link{ShortCode: "qrd1", Source: model.LinkSourceQR})
	createQR(t, db, links[0], "")

	second := &model.QRCode{UserID: links[0].UserID, GroupID: links[0].GroupID, LinkID: links[0].ID}
	err := repository.NewQRRepository(db).Create(context.Background(), second)
	assert.ErrorIs(t, err, repository.ErrQRCodeExistsForLink)
}

func TestQRRepository_ListByGroup_FiltersAndScans(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "qrlist@example.com",
		&model.Link{ShortCode: "qrl1", Source: model.LinkSourceLink},
		&model.Link{ShortCode: "qrl2", Source: model.LinkSourceQR})
	_, foreign := linkFixture(t, db, "qrlist-other@example.com", &model.Link{ShortCode: "qrlf", Source: model.LinkSourceQR})
	repo := repository.NewQRRepository(db)
	withLink := createQR(t, db, links[0], "Poster")
	scanned := createQR(t, db, links[1], "Flyer")
	createQR(t, db, foreign[0], "Foreign")
	require.NoError(t, repo.SetTags(ctx, withLink.ID, []string{"spring"}))
	for _, src := range []string{"qr", "qr", "link"} {
		_, err := db.ExecContext(ctx, `INSERT INTO clicks (link_id, source) VALUES (?, ?)`, links[1].ID, src)
		require.NoError(t, err)
	}

	// Default: newest first, scans count only source='qr' clicks.
	rows, total, err := repo.ListByGroup(ctx, group.ID, repository.QRFilter{}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.ElementsMatch(t, []uint64{withLink.ID, scanned.ID}, qrIDsOf(rows))
	for _, r := range rows {
		if r.ID == scanned.ID {
			assert.Equal(t, int64(2), r.TotalScans)
		} else {
			assert.Equal(t, int64(0), r.TotalScans)
		}
	}

	rows, _, err = repo.ListByGroup(ctx, group.ID, repository.QRFilter{Sort: "scans"}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []uint64{scanned.ID, withLink.ID}, qrIDsOf(rows))

	rows, total, err = repo.ListByGroup(ctx, group.ID, repository.QRFilter{HasLink: boolPtr(true)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, []uint64{withLink.ID}, qrIDsOf(rows))
	rows, _, err = repo.ListByGroup(ctx, group.ID, repository.QRFilter{HasLink: boolPtr(false)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []uint64{scanned.ID}, qrIDsOf(rows))

	// Query matches a QR tag, the QR title and the link's short code.
	for q, want := range map[string]uint64{"spring": withLink.ID, "flyer": scanned.ID, "qrl2": scanned.ID} {
		rows, total, err = repo.ListByGroup(ctx, group.ID, repository.QRFilter{Query: q}, 20, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total, q)
		assert.Equal(t, []uint64{want}, qrIDsOf(rows), q)
	}

	rows, _, err = repo.ListByGroup(ctx, group.ID, repository.QRFilter{Tags: []string{"spring"}}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []uint64{withLink.ID}, qrIDsOf(rows))

	require.NoError(t, repo.SetArchived(ctx, []uint64{scanned.ID}, true))
	rows, _, err = repo.ListByGroup(ctx, group.ID, repository.QRFilter{Archived: boolPtr(true)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []uint64{scanned.ID}, qrIDsOf(rows))

	count, err := repo.CountByGroup(ctx, group.ID, repository.QRFilter{Query: "poster"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	count, err = repo.CountByGroup(ctx, group.ID, repository.QRFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestQRRepository_Tags_SetListBulk(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "qrtags@example.com",
		&model.Link{ShortCode: "qrt1", Source: model.LinkSourceQR},
		&model.Link{ShortCode: "qrt2", Source: model.LinkSourceQR})
	repo := repository.NewQRRepository(db)
	a := createQR(t, db, links[0], "")
	b := createQR(t, db, links[1], "")

	require.NoError(t, repo.SetTags(ctx, a.ID, []string{" news ", "news", "", "Promo"}))
	require.NoError(t, repo.SetTags(ctx, b.ID, []string{"news"}))
	got, err := repo.GetTagsFor(ctx, []uint64{a.ID, b.ID})
	require.NoError(t, err)
	assert.Equal(t, []string{"news", "Promo"}, got[a.ID])
	assert.Equal(t, []string{"news"}, got[b.ID])

	tags, err := repo.ListTagsByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Promo", "news"}, tags)

	require.NoError(t, repo.AddTags(ctx, []uint64{a.ID, b.ID}, []string{"bulk", "news"}))
	require.NoError(t, repo.RemoveTags(ctx, []uint64{a.ID, b.ID}, []string{"news"}))
	got, err = repo.GetTagsFor(ctx, []uint64{a.ID, b.ID})
	require.NoError(t, err)
	assert.Equal(t, []string{"bulk", "Promo"}, got[a.ID])
	assert.Equal(t, []string{"bulk"}, got[b.ID])

	// SetTags with nil clears; untagged QR codes are absent from the map.
	require.NoError(t, repo.SetTags(ctx, b.ID, nil))
	got, err = repo.GetTagsFor(ctx, []uint64{b.ID})
	require.NoError(t, err)
	assert.Empty(t, got)

	empty, err := repo.ListTagsByGroup(ctx, group.ID+1000)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestQRRepository_QRCodeIDsForLinks(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	_, links := linkFixture(t, db, "qrids@example.com",
		&model.Link{ShortCode: "qri1"}, &model.Link{ShortCode: "qri2"})
	repo := repository.NewQRRepository(db)
	qr := createQR(t, db, links[0], "")

	got, err := repo.QRCodeIDsForLinks(ctx, []uint64{links[0].ID, links[1].ID})
	require.NoError(t, err)
	assert.Equal(t, map[uint64]uint64{links[0].ID: qr.ID}, got)

	got, err = repo.QRCodeIDsForLinks(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
