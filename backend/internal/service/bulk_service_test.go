package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	svcmocks "github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const bulkHeader = "long_url,title,custom_back_half,tags,domain\n"

func requireValidation(t *testing.T, err error, contains string) {
	t.Helper()
	var ve *service.ValidationError
	require.True(t, errors.As(err, &ve), "want ValidationError, got %v", err)
	assert.Contains(t, ve.Msg, contains)
}

func TestParseBulkCSV(t *testing.T) {
	t.Run("good file", func(t *testing.T) {
		in := "\ufeffLong_URL, Title ,custom_back_half,TAGS,domain\n" +
			"https://a.example/1,Plain,,,\n" +
			"\n" +
			",,,,\n" +
			`https://b.example/2,"Hello, world",promo, a | b ,links.example.com` + "\n" +
			"https://c.example/3\n"
		rows, err := service.ParseBulkCSV(strings.NewReader(in))
		require.NoError(t, err)
		require.Len(t, rows, 3, "blank lines skipped")
		assert.Equal(t, service.BulkRow{Line: 1, LongURL: "https://a.example/1", Title: "Plain", Tags: []string{}}, rows[0])
		assert.Equal(t, service.BulkRow{Line: 2, LongURL: "https://b.example/2", Title: "Hello, world",
			CustomBackHalf: "promo", Tags: []string{"a", "b"}, Domain: "links.example.com"}, rows[1])
		assert.Equal(t, 3, rows[2].Line, "short rows are padded")
		assert.Equal(t, "https://c.example/3", rows[2].LongURL)
	})
	t.Run("wrong header", func(t *testing.T) {
		_, err := service.ParseBulkCSV(strings.NewReader("url,title,custom_back_half,tags,domain\nhttps://a.example,,,,\n"))
		requireValidation(t, err, "header")
		_, err = service.ParseBulkCSV(strings.NewReader("title,long_url,custom_back_half,tags,domain\nx,https://a.example,,,\n"))
		requireValidation(t, err, "header")
	})
	t.Run("too many rows", func(t *testing.T) {
		var b strings.Builder
		b.WriteString(bulkHeader)
		for i := 0; i < 1001; i++ {
			fmt.Fprintf(&b, "https://a.example/%d,,,,\n", i)
		}
		_, err := service.ParseBulkCSV(strings.NewReader(b.String()))
		requireValidation(t, err, "at most 1000 rows")
	})
	t.Run("exactly 1000 rows", func(t *testing.T) {
		var b strings.Builder
		b.WriteString(bulkHeader)
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(&b, "https://a.example/%d,,,,\n", i)
		}
		rows, err := service.ParseBulkCSV(strings.NewReader(b.String()))
		require.NoError(t, err)
		assert.Len(t, rows, 1000)
		assert.Equal(t, 1000, rows[999].Line)
	})
	t.Run("no rows", func(t *testing.T) {
		_, err := service.ParseBulkCSV(strings.NewReader(""))
		requireValidation(t, err, "no rows")
		_, err = service.ParseBulkCSV(strings.NewReader(bulkHeader + "\n,,,,\n"))
		requireValidation(t, err, "no rows")
	})
	t.Run("unterminated quote", func(t *testing.T) {
		_, err := service.ParseBulkCSV(strings.NewReader(bulkHeader +
			"https://a.example,\"unterminated,,,\nhttps://b.example,,,,\nhttps://c.example,,,,\n"))
		requireValidation(t, err, "invalid CSV on line")
	})
	t.Run("bare quote in unquoted field", func(t *testing.T) {
		_, err := service.ParseBulkCSV(strings.NewReader(bulkHeader + "https://a.example,say \"hi\",,,\n"))
		requireValidation(t, err, "line 2")
	})
	t.Run("quoted comma and doubled quotes", func(t *testing.T) {
		rows, err := service.ParseBulkCSV(strings.NewReader(bulkHeader +
			"https://a.example,\"He said \"\"hi\"\", then left\",,,\n"))
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, `He said "hi", then left`, rows[0].Title)
	})
	t.Run("too many columns", func(t *testing.T) {
		_, err := service.ParseBulkCSV(strings.NewReader(bulkHeader + "https://a.example,,,,,extra\n"))
		requireValidation(t, err, "line 2")
		rows, err := service.ParseBulkCSV(strings.NewReader(bulkHeader + "https://a.example,,,,,,\n"))
		require.NoError(t, err, "trailing empty cells are tolerated")
		assert.Len(t, rows, 1)
	})
}

type bulkSvcFixture struct {
	svc      *service.BulkServiceImpl
	repo     *mocks.MockBulkUploadRepository
	activity *svcmocks.MockActivityService
}

func newBulkSvc(t *testing.T) *bulkSvcFixture {
	ctrl := gomock.NewController(t)
	f := &bulkSvcFixture{repo: mocks.NewMockBulkUploadRepository(ctrl), activity: svcmocks.NewMockActivityService(ctrl)}
	f.svc = service.NewBulkService(service.BulkServiceDeps{Bulk: f.repo, Activity: f.activity})
	return f
}

func TestBulkService_Create(t *testing.T) {
	f := newBulkSvc(t)
	ctx := context.Background()
	csv := bulkHeader + "https://a.example,,,,\nhttps://b.example,,,,\n"
	f.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, b *model.BulkUpload) error {
		assert.Equal(t, uint64(1), b.UserID)
		assert.Equal(t, uint64(7), b.GroupID)
		assert.Equal(t, "links.csv", b.Filename, "path stripped")
		assert.Equal(t, model.BulkStatusQueued, b.Status)
		assert.Equal(t, 2, b.Total)
		assert.Equal(t, csv, b.SourceCSV)
		b.ID = 9
		return nil
	})
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionBulkCreate, service.EntityBulk, uint64(9),
		map[string]any{"filename": "links.csv", "total": 2})
	b, err := f.svc.Create(ctx, 1, 7, `C:\Users\me\links.csv`, []byte(csv))
	require.NoError(t, err)
	assert.Equal(t, uint64(9), b.ID)
}

func TestBulkService_CreateFilenameSanitised(t *testing.T) {
	f := newBulkSvc(t)
	var names []string
	f.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, b *model.BulkUpload) error {
		names = append(names, b.Filename)
		return nil
	}).Times(3)
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(3)
	csv := []byte(bulkHeader + "https://a.example,,,,\n")
	for _, n := range []string{"../../etc/x\x00.csv", "", strings.Repeat("é", 300) + ".csv"} {
		_, err := f.svc.Create(context.Background(), 1, 7, n, csv)
		require.NoError(t, err)
	}
	assert.Equal(t, "x.csv", names[0])
	assert.Equal(t, "upload.csv", names[1])
	assert.Equal(t, 255, len([]rune(names[2])))
}

func TestBulkService_ParseLimits(t *testing.T) {
	f := newBulkSvc(t)
	ctx := context.Background()
	big := bytes.Repeat([]byte("a"), 2<<20+1)
	_, err := f.svc.Create(ctx, 1, 7, "big.csv", big)
	requireValidation(t, err, "larger than 2 MB")

	_, err = f.svc.Create(ctx, 1, 7, "bad.csv", []byte("nope\n"))
	requireValidation(t, err, "header")

	// Windows-1252 "Café" (0xE9 is not valid UTF-8).
	latin1 := []byte("long_url,title,custom_back_half,tags,domain\nhttps://a.example,Caf\xe9,,,\n")
	_, err = f.svc.Create(ctx, 1, 7, "latin1.csv", latin1)
	requireValidation(t, err, "must be UTF-8 encoded")
}

func TestBulkService_WrongGroup(t *testing.T) {
	f := newBulkSvc(t)
	ctx := context.Background()
	f.repo.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.BulkUpload{ID: 9, GroupID: 8, Status: model.BulkStatusDone}, nil).Times(2)
	_, err := f.svc.Get(ctx, 7, 9)
	assert.ErrorIs(t, err, repository.ErrBulkUploadNotFound)
	_, err = f.svc.Results(ctx, 7, 9)
	assert.ErrorIs(t, err, repository.ErrBulkUploadNotFound)

	f.repo.EXPECT().GetByID(gomock.Any(), uint64(10)).Return(nil, repository.ErrBulkUploadNotFound)
	_, err = f.svc.Get(ctx, 7, 10)
	assert.ErrorIs(t, err, repository.ErrBulkUploadNotFound)
}

func TestBulkService_Results_NotFinished409(t *testing.T) {
	f := newBulkSvc(t)
	ctx := context.Background()
	for _, st := range []string{model.BulkStatusQueued, model.BulkStatusProcessing} {
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.BulkUpload{ID: 9, GroupID: 7, Status: st}, nil)
		_, err := f.svc.Results(ctx, 7, 9)
		assert.ErrorIs(t, err, service.ErrBulkNotFinished, st)
	}
	res := "row,long_url,short_url,status,error\n1,https://a.example,http://s/x,ok,\n"
	f.repo.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.BulkUpload{ID: 9, GroupID: 7, Status: model.BulkStatusDone, ResultCSV: &res}, nil)
	got, err := f.svc.Results(ctx, 7, 9)
	require.NoError(t, err)
	assert.Equal(t, res, got)

	f.repo.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.BulkUpload{ID: 9, GroupID: 7, Status: model.BulkStatusFailed}, nil)
	got, err = f.svc.Results(ctx, 7, 9)
	require.NoError(t, err)
	assert.Equal(t, "row,long_url,short_url,status,error\n", got, "failed job without results yields the header")
}

func TestBulkService_List(t *testing.T) {
	f := newBulkSvc(t)
	f.repo.EXPECT().ListByGroup(gomock.Any(), uint64(7)).Return([]model.BulkUpload{{ID: 1}}, nil)
	list, err := f.svc.List(context.Background(), 7)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestEncodeBulkResults_CSVSafe(t *testing.T) {
	out, err := service.EncodeBulkResults([]service.BulkResultRow{
		{Row: 1, LongURL: "https://a.example/?q=1,2", ShortURL: "http://s/x", Status: "ok"},
		{Row: 2, LongURL: "=cmd|' /C calc'!A0", Status: "error", Error: "-bad"},
	})
	require.NoError(t, err)
	assert.Equal(t, "row,long_url,short_url,status,error\n"+
		"1,\"https://a.example/?q=1,2\",http://s/x,ok,\n"+
		"2,'=cmd|' /C calc'!A0,,error,'-bad\n", out)
}
