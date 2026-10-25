package worker

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	svcmocks "github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const bulkTestHeader = "long_url,title,custom_back_half,tags,domain\n"

type progressCall struct{ processed, succeeded, failed int }

// fakeBulkRepo is an in-memory repository.BulkUploadRepository.
type fakeBulkRepo struct {
	mu       sync.Mutex
	jobs     map[uint64]*model.BulkUpload
	order    []uint64
	progress map[uint64][]progressCall
	reclaims []time.Duration
}

var _ repository.BulkUploadRepository = (*fakeBulkRepo)(nil)

func newFakeBulkRepo(jobs ...model.BulkUpload) *fakeBulkRepo {
	f := &fakeBulkRepo{jobs: map[uint64]*model.BulkUpload{}, progress: map[uint64][]progressCall{}}
	for i := range jobs {
		j := jobs[i]
		if j.Status == "" {
			j.Status = model.BulkStatusQueued
		}
		f.jobs[j.ID] = &j
		f.order = append(f.order, j.ID)
	}
	return f
}

func (f *fakeBulkRepo) Create(_ context.Context, b *model.BulkUpload) error { panic("unused") }

func (f *fakeBulkRepo) GetByID(_ context.Context, id uint64) (*model.BulkUpload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return nil, repository.ErrBulkUploadNotFound
	}
	c := *j
	return &c, nil
}

func (f *fakeBulkRepo) ListByGroup(context.Context, uint64) ([]model.BulkUpload, error) {
	panic("unused")
}

func (f *fakeBulkRepo) ClaimNext(context.Context) (*model.BulkUpload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.order {
		if j := f.jobs[id]; j.Status == model.BulkStatusQueued {
			j.Status = model.BulkStatusProcessing
			c := *j
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeBulkRepo) ReclaimStale(_ context.Context, olderThan time.Duration) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reclaims = append(f.reclaims, olderThan)
	return 0, nil
}

func (f *fakeBulkRepo) UpdateProgress(_ context.Context, id uint64, processed, succeeded, failed int, partial string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return repository.ErrBulkUploadNotFound
	}
	j.Processed, j.Succeeded, j.Failed = processed, succeeded, failed
	if partial != "" {
		j.ResultCSV = &partial
	}
	f.progress[id] = append(f.progress[id], progressCall{processed, succeeded, failed})
	return nil
}

func (f *fakeBulkRepo) Finish(_ context.Context, id uint64, status, resultCSV, errMsg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return repository.ErrBulkUploadNotFound
	}
	j.Status, j.Error = status, errMsg
	if resultCSV != "" {
		j.ResultCSV = &resultCSV
	}
	return nil
}

type bulkWorkerFixture struct {
	repo    *fakeBulkRepo
	links   *svcmocks.MockLinkService
	domains *mocks.MockDomainRepository
	w       *BulkWorker
}

func newBulkWorkerFixture(t *testing.T, batchSize int, domains []*model.Domain, jobs ...model.BulkUpload) *bulkWorkerFixture {
	ctrl := gomock.NewController(t)
	f := &bulkWorkerFixture{
		repo:    newFakeBulkRepo(jobs...),
		links:   svcmocks.NewMockLinkService(ctrl),
		domains: mocks.NewMockDomainRepository(ctrl),
	}
	f.domains.EXPECT().ListByUserID(gomock.Any(), gomock.Any()).Return(domains, nil).AnyTimes()
	f.w = NewBulkWorker(f.repo, f.links, f.domains, time.Hour, batchSize, "http://short.test")
	return f
}

func bulkJob(id uint64, csvBody string) model.BulkUpload {
	return model.BulkUpload{ID: id, UserID: 1, GroupID: 7, Filename: "x.csv", SourceCSV: bulkTestHeader + csvBody}
}

func readResults(t *testing.T, j *model.BulkUpload) [][]string {
	t.Helper()
	require.NotNil(t, j.ResultCSV)
	recs, err := csv.NewReader(strings.NewReader(*j.ResultCSV)).ReadAll()
	require.NoError(t, err)
	require.Equal(t, []string{"row", "long_url", "short_url", "status", "error"}, recs[0])
	return recs[1:]
}

// okCreate returns a Create stub that assigns sequential short codes.
func okCreate() func(context.Context, uint64, service.CreateLinkInput) (*model.Link, error) {
	var mu sync.Mutex
	n := 0
	return func(_ context.Context, _ uint64, in service.CreateLinkInput) (*model.Link, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		code := in.CustomCode
		if code == "" {
			code = fmt.Sprintf("c%d", n)
		}
		return &model.Link{ID: uint64(n), GroupID: in.GroupID, ShortCode: code, OriginalURL: in.OriginalURL, DomainID: in.DomainID}, nil
	}
}

func TestBulkWorker_RowErrorsDoNotAbort(t *testing.T) {
	f := newBulkWorkerFixture(t, 50, nil, bulkJob(5,
		"https://a.example/1,One,,x|y,\n"+
			"https://a.example/2,Two,taken,,\n"+
			"https://a.example/3,Three,,,\n"))
	create := okCreate()
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
		func(ctx context.Context, uid uint64, in service.CreateLinkInput) (*model.Link, error) {
			assert.Equal(t, uint64(7), in.GroupID)
			assert.Nil(t, in.DomainID, "empty domain uses the group default")
			if in.CustomCode == "taken" {
				return nil, service.ErrShortCodeTaken
			}
			if in.OriginalURL == "https://a.example/1" {
				assert.Equal(t, "One", in.Title)
				assert.Equal(t, []string{"x", "y"}, in.Tags)
			}
			return create(ctx, uid, in)
		}).Times(3)

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusDone, j.Status)
	assert.Equal(t, []progressCall{{3, 2, 1}}, f.repo.progress[5], "one progress update per batch")
	assert.Equal(t, []time.Duration{10 * time.Minute}, f.repo.reclaims, "stale jobs reclaimed once per poll")
	rows := readResults(t, j)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"1", "https://a.example/1", "http://short.test/c1", "ok", ""}, rows[0])
	assert.Equal(t, "2", rows[1][0])
	assert.Equal(t, "", rows[1][2])
	assert.Equal(t, "error", rows[1][3])
	assert.Contains(t, rows[1][4], "already taken")
	assert.Equal(t, "ok", rows[2][3])
}

func TestBulkWorker_BatchesOf50(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&b, "https://a.example/%d,t,,,\n", i)
	}
	f := newBulkWorkerFixture(t, 50, nil, bulkJob(5, b.String()))
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(okCreate()).Times(120)

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusDone, j.Status)
	assert.Equal(t, []progressCall{{50, 50, 0}, {100, 100, 0}, {120, 120, 0}}, f.repo.progress[5])
	assert.Len(t, readResults(t, j), 120)
}

func TestBulkWorker_UnknownDomainRowError(t *testing.T) {
	domains := []*model.Domain{
		{ID: 3, UserID: 1, Domain: "Links.Example.com", VerificationStatus: model.DomainVerificationOK},
		{ID: 4, UserID: 1, Domain: "pending.example.com", VerificationStatus: model.DomainVerificationPending},
	}
	f := newBulkWorkerFixture(t, 50, domains, bulkJob(5,
		"https://a.example/1,,,,links.example.COM\n"+
			"https://a.example/2,,,,pending.example.com\n"+
			"https://a.example/3,,,,nope.example.com\n"))
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
		func(ctx context.Context, uid uint64, in service.CreateLinkInput) (*model.Link, error) {
			require.NotNil(t, in.DomainID)
			assert.Equal(t, uint64(3), *in.DomainID)
			return okCreate()(ctx, uid, in)
		}).Times(1)

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusDone, j.Status)
	rows := readResults(t, j)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"1", "https://a.example/1", "http://Links.Example.com/c1", "ok", ""}, rows[0])
	for _, r := range rows[1:] {
		assert.Equal(t, "error", r[3])
		assert.Equal(t, "unknown or unverified domain", r[4])
	}
	assert.Equal(t, []progressCall{{3, 1, 2}}, f.repo.progress[5])
}

func TestBulkWorker_CSVSafe(t *testing.T) {
	f := newBulkWorkerFixture(t, 50, nil, bulkJob(5, "=cmd|' /C calc'!A0,=title,,,\n"))
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).Return(nil, service.ErrInvalidURL)

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Contains(t, *j.ResultCSV, "'=cmd")
	rows := readResults(t, j)
	assert.Equal(t, "'=cmd|' /C calc'!A0", rows[0][1])
	assert.Equal(t, "error", rows[0][3])
}

func TestBulkWorker_UnexpectedErrorIsGeneric(t *testing.T) {
	f := newBulkWorkerFixture(t, 50, nil, bulkJob(5, "https://a.example/1,,,,\n"))
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).
		Return(nil, fmt.Errorf("Error 1213: Deadlock found when trying to get lock"))

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	rows := readResults(t, j)
	assert.Equal(t, "failed to create link", rows[0][4], "driver errors are not leaked into the CSV")
}

func TestBulkWorker_ParseErrorFailsJob(t *testing.T) {
	f := newBulkWorkerFixture(t, 50, nil, model.BulkUpload{ID: 5, UserID: 1, GroupID: 7, SourceCSV: "bad,header\n"},
		bulkJob(6, "https://a.example/1,,,,\n"))
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(okCreate())

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusFailed, j.Status)
	assert.Contains(t, j.Error, "header")
	assert.Nil(t, j.ResultCSV)
	j, _ = f.repo.GetByID(context.Background(), 6)
	assert.Equal(t, model.BulkStatusDone, j.Status, "the next job still runs")
}

func TestBulkWorker_PanicFailsJob(t *testing.T) {
	f := newBulkWorkerFixture(t, 50, nil, bulkJob(5, "https://a.example/1,,,,\n"), bulkJob(6, "https://a.example/2,,,,\n"))
	gomock.InOrder(
		f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
			func(context.Context, uint64, service.CreateLinkInput) (*model.Link, error) { panic("boom") }),
		f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(okCreate()),
	)

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusFailed, j.Status)
	assert.Equal(t, "internal error", j.Error)
	j, _ = f.repo.GetByID(context.Background(), 6)
	assert.Equal(t, model.BulkStatusDone, j.Status)
}

func TestBulkWorker_StartStop(t *testing.T) {
	f := newBulkWorkerFixture(t, 50, nil, bulkJob(5, "https://a.example/1,,,,\n"))
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(okCreate())
	f.w = NewBulkWorker(f.repo, f.links, f.domains, 10*time.Millisecond, 50, "http://short.test")
	f.w.Start()
	require.Eventually(t, func() bool {
		j, _ := f.repo.GetByID(context.Background(), 5)
		return j.Status == model.BulkStatusDone
	}, 2*time.Second, 10*time.Millisecond)
	f.w.Stop()
}

func TestBulkWorker_StopBetweenBatchesFinishesWithPartialResults(t *testing.T) {
	f := newBulkWorkerFixture(t, 1, nil, bulkJob(5, "https://a.example/1,,,,\nhttps://a.example/2,,,,\n"),
		bulkJob(6, "https://a.example/3,,,,\n"))
	create := okCreate()
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
		func(ctx context.Context, uid uint64, in service.CreateLinkInput) (*model.Link, error) {
			f.w.signalStop() // shutdown arrives while the first batch runs
			return create(ctx, uid, in)
		}).Times(1)

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusFailed, j.Status)
	assert.Equal(t, "interrupted by server shutdown", j.Error)
	assert.Equal(t, []progressCall{{1, 1, 0}}, f.repo.progress[5])
	rows := readResults(t, j)
	require.Len(t, rows, 1, "partial results are kept")
	assert.Equal(t, "ok", rows[0][3])
	j, _ = f.repo.GetByID(context.Background(), 6)
	assert.Equal(t, model.BulkStatusQueued, j.Status, "no new job is claimed while stopping")
}

func TestBulkWorker_StopIsIdempotent(t *testing.T) {
	f := newBulkWorkerFixture(t, 50, nil)
	f.w = NewBulkWorker(f.repo, f.links, f.domains, time.Hour, 50, "http://short.test")
	f.w.Start()
	f.w.Stop()
	f.w.Stop()
}

func TestBulkWorker_StoresPartialResultsWithProgress(t *testing.T) {
	f := newBulkWorkerFixture(t, 1, nil, bulkJob(5, "https://a.example/1,,,,\nhttps://a.example/2,,,,\n"))
	var partials []string
	create := okCreate()
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
		func(ctx context.Context, uid uint64, in service.CreateLinkInput) (*model.Link, error) {
			j, _ := f.repo.GetByID(ctx, 5)
			if j.ResultCSV != nil {
				partials = append(partials, *j.ResultCSV)
			}
			return create(ctx, uid, in)
		}).Times(2)

	f.w.poll()

	require.Len(t, partials, 1, "the first batch's results are stored before the second batch runs")
	assert.Equal(t, "row,long_url,short_url,status,error\n1,https://a.example/1,http://short.test/c1,ok,\n", partials[0])
}

func TestBulkWorker_ResumesFromProcessed(t *testing.T) {
	// A reclaimed job: rows 1-2 were processed (one ok, one error) before
	// the crash; rows 3-4 remain.
	stored := "row,long_url,short_url,status,error\n" +
		"1,https://a.example/1,http://short.test/one,ok,\n" +
		"2,https://a.example/2,,error,that back-half is already taken\n"
	job := bulkJob(5, "https://a.example/1,,one,,\nhttps://a.example/2,,taken,,\nhttps://a.example/3,,,,\nhttps://a.example/4,,,,\n")
	job.Total, job.Processed, job.Succeeded, job.Failed, job.ResultCSV = 4, 2, 1, 1, &stored
	f := newBulkWorkerFixture(t, 50, nil, job)
	var created []string
	create := okCreate()
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
		func(ctx context.Context, uid uint64, in service.CreateLinkInput) (*model.Link, error) {
			created = append(created, in.OriginalURL)
			return create(ctx, uid, in)
		}).Times(2)

	f.w.poll()

	assert.Equal(t, []string{"https://a.example/3", "https://a.example/4"}, created, "processed rows are skipped")
	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusDone, j.Status)
	assert.Equal(t, []progressCall{{4, 3, 1}}, f.repo.progress[5])
	rows := readResults(t, j)
	require.Len(t, rows, 4, "new results are appended to the stored ones")
	assert.Equal(t, []string{"1", "https://a.example/1", "http://short.test/one", "ok", ""}, rows[0])
	assert.Equal(t, "error", rows[1][3])
	assert.Equal(t, []string{"3", "https://a.example/3", "http://short.test/c1", "ok", ""}, rows[2])
	assert.Equal(t, "4", rows[3][0])
}

func TestBulkWorker_ProcessedWithoutResultsStartsOver(t *testing.T) {
	job := bulkJob(5, "https://a.example/1,,,,\nhttps://a.example/2,,,,\n")
	job.Processed, job.Succeeded = 1, 1 // e.g. reclaimed before partial results were stored
	f := newBulkWorkerFixture(t, 50, nil, job)
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(okCreate()).Times(2)

	f.w.poll()

	j, _ := f.repo.GetByID(context.Background(), 5)
	assert.Equal(t, model.BulkStatusDone, j.Status)
	assert.Equal(t, []progressCall{{2, 2, 0}}, f.repo.progress[5])
	assert.Len(t, readResults(t, j), 2)
}
