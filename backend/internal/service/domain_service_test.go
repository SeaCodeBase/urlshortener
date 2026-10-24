package service_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// fakeResolver answers DNS lookups from maps; missing names fail.
type fakeResolver struct {
	txt   map[string][]string
	cname map[string]string
}

func (f *fakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if v, ok := f.txt[name]; ok {
		return v, nil
	}
	return nil, errors.New("no such host")
}

func (f *fakeResolver) LookupCNAME(_ context.Context, name string) (string, error) {
	if v, ok := f.cname[name]; ok {
		return v, nil
	}
	return "", errors.New("no such host")
}

type domainSvcDeps struct {
	svc     *service.DomainServiceImpl
	domains *mocks.MockDomainRepository
	groups  *mocks.MockGroupRepository
	dns     *fakeResolver
}

func newDomainService(t *testing.T) *domainSvcDeps {
	ctrl := gomock.NewController(t)
	d := &domainSvcDeps{
		domains: mocks.NewMockDomainRepository(ctrl),
		groups:  mocks.NewMockGroupRepository(ctrl),
		dns:     &fakeResolver{txt: map[string][]string{}, cname: map[string]string{}},
	}
	d.svc = service.NewDomainService(d.domains, d.groups, d.dns, "sho.rt")
	return d
}

const testToken = "0123456789abcdef0123456789abcdef"

func pendingDomain() *model.Domain {
	return &model.Domain{ID: 3, UserID: 1, Domain: "go.example.com", VerificationToken: testToken,
		VerificationStatus: model.DomainVerificationPending}
}

func TestDomainService_Create_GeneratesTokenAndPending(t *testing.T) {
	d := newDomainService(t)
	var created *model.Domain
	d.domains.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, dom *model.Domain) error {
		created = dom
		dom.ID = 3
		return nil
	})
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).DoAndReturn(func(context.Context, uint64) (*model.Domain, error) {
		return created, nil
	})

	got, err := d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: "  Go.Example.COM. "})
	require.NoError(t, err)
	assert.Equal(t, "go.example.com", got.Domain)
	assert.Equal(t, uint64(1), got.UserID)
	assert.Equal(t, model.DomainVerificationPending, got.VerificationStatus)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{32}$`), got.VerificationToken)
	assert.Nil(t, got.GroupID)
}

func TestDomainService_Create_TokensAreRandom(t *testing.T) {
	d := newDomainService(t)
	tokens := map[string]bool{}
	d.domains.EXPECT().Create(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, dom *model.Domain) error {
		tokens[dom.VerificationToken] = true
		return nil
	})
	d.domains.EXPECT().GetByID(gomock.Any(), gomock.Any()).Times(2).Return(pendingDomain(), nil)
	_, err := d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: "a.example.com"})
	require.NoError(t, err)
	_, err = d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: "b.example.com"})
	require.NoError(t, err)
	assert.Len(t, tokens, 2)
}

func TestDomainService_Create_InvalidHostnames(t *testing.T) {
	d := newDomainService(t)
	long := strings.Repeat("a", 63) + "."
	for _, in := range []string{
		"",
		"localhost",
		"https://go.example.com",
		"go.example.com/path",
		"go.example.com:8080",
		"-bad.example.com",
		"bad-.example.com",
		"under_score.example.com",
		"two..dots.com",
		strings.Repeat("a", 64) + ".com",
		strings.Repeat(long, 4) + "com", // 4*64+3 = 259 > 253
		"1.2.3.4",
		"exa mple.com",
	} {
		_, err := d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: in})
		assert.ErrorIs(t, err, service.ErrInvalidDomain, "input %q", in)
	}
}

func TestDomainService_Create_Duplicate(t *testing.T) {
	d := newDomainService(t)
	d.domains.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrDomainExists)
	_, err := d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: "go.example.com"})
	assert.ErrorIs(t, err, service.ErrDomainExists)
}

func TestDomainService_Create_GroupMustBeOwned(t *testing.T) {
	d := newDomainService(t)
	d.groups.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.Group{ID: 9, UserID: 2}, nil)
	_, err := d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: "go.example.com", GroupID: u64(9)})
	assert.ErrorIs(t, err, service.ErrGroupNotFound)

	d.groups.EXPECT().GetByID(gomock.Any(), uint64(10)).Return(nil, repository.ErrGroupNotFound)
	_, err = d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: "go.example.com", GroupID: u64(10)})
	assert.ErrorIs(t, err, service.ErrGroupNotFound)
}

func TestDomainService_Create_WithOwnedGroup(t *testing.T) {
	d := newDomainService(t)
	d.groups.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.Group{ID: 9, UserID: 1}, nil)
	d.domains.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, dom *model.Domain) error {
		require.NotNil(t, dom.GroupID)
		assert.Equal(t, uint64(9), *dom.GroupID)
		dom.ID = 3
		return nil
	})
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
	_, err := d.svc.Create(context.Background(), 1, service.CreateDomainInput{Domain: "go.example.com", GroupID: u64(9)})
	require.NoError(t, err)
}

func expectVerifyUpdate(d *domainSvcDeps) *model.Domain {
	saved := &model.Domain{}
	d.domains.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, dom *model.Domain) error {
		*saved = *dom
		return nil
	})
	return saved
}

func TestDomainService_Verify_TXT(t *testing.T) {
	d := newDomainService(t)
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
	d.dns.txt["_urlshortener.go.example.com"] = []string{"v=spf1 -all", "verify=" + testToken}
	saved := expectVerifyUpdate(d)

	before := time.Now().Add(-time.Second)
	got, err := d.svc.Verify(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, model.DomainVerificationOK, got.VerificationStatus)
	assert.Nil(t, got.VerificationError)
	require.True(t, got.VerifiedAt.Valid)
	require.True(t, got.LastCheckedAt.Valid)
	assert.True(t, got.LastCheckedAt.Time.After(before))
	assert.Equal(t, model.DomainVerificationOK, saved.VerificationStatus)
}

func TestDomainService_Verify_CNAMEWithTrailingDot(t *testing.T) {
	d := newDomainService(t)
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
	d.dns.cname["go.example.com"] = "SHO.RT."
	expectVerifyUpdate(d)

	got, err := d.svc.Verify(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, model.DomainVerificationOK, got.VerificationStatus)
}

func TestDomainService_Verify_FailureRecordsError(t *testing.T) {
	d := newDomainService(t)
	prev := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dom := pendingDomain()
	dom.VerifiedAt = model.NullTime{NullTime: sql.NullTime{Time: prev, Valid: true}}
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(dom, nil)
	d.dns.cname["go.example.com"] = "elsewhere.net."
	saved := expectVerifyUpdate(d)

	got, err := d.svc.Verify(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, model.DomainVerificationError, got.VerificationStatus)
	require.NotNil(t, got.VerificationError)
	assert.Contains(t, *got.VerificationError, "TXT record not found")
	assert.True(t, got.LastCheckedAt.Valid)
	// verified_at keeps the last successful verification.
	assert.True(t, got.VerifiedAt.Time.Equal(prev))
	assert.Equal(t, model.DomainVerificationError, saved.VerificationStatus)
}

func TestDomainService_Verify_WrongTXTValue(t *testing.T) {
	d := newDomainService(t)
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
	d.dns.txt["_urlshortener.go.example.com"] = []string{"verify=nottherighttoken"}
	expectVerifyUpdate(d)

	got, err := d.svc.Verify(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, model.DomainVerificationError, got.VerificationStatus)
	require.NotNil(t, got.VerificationError)
	assert.Contains(t, *got.VerificationError, "TXT record")
}

func TestDomainService_Verify_ForeignOrMissing(t *testing.T) {
	d := newDomainService(t)
	foreign := pendingDomain()
	foreign.UserID = 2
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(foreign, nil)
	_, err := d.svc.Verify(context.Background(), 1, 3)
	assert.ErrorIs(t, err, service.ErrNotDomainOwner)

	d.domains.EXPECT().GetByID(gomock.Any(), uint64(4)).Return(nil, repository.ErrDomainNotFound)
	_, err = d.svc.Verify(context.Background(), 1, 4)
	assert.ErrorIs(t, err, service.ErrNotDomainOwner)
}

func sp(s string) *string { return &s }

func TestDomainService_Update_Redirects(t *testing.T) {
	d := newDomainService(t)
	dom := pendingDomain()
	dom.NotFoundRedirect = sp("https://old.example.com/404")
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(dom, nil)
	saved := expectVerifyUpdate(d)

	got, err := d.svc.Update(context.Background(), 1, 3, service.UpdateDomainInput{
		RootRedirect:     sp(" https://example.com/home "),
		NotFoundRedirect: sp(""),
	})
	require.NoError(t, err)
	require.NotNil(t, got.RootRedirect)
	assert.Equal(t, "https://example.com/home", *got.RootRedirect)
	assert.Nil(t, got.NotFoundRedirect, "empty string clears")
	assert.Equal(t, got.RootRedirect, saved.RootRedirect)
}

func TestDomainService_Update_InvalidRedirect(t *testing.T) {
	d := newDomainService(t)
	for _, bad := range []string{"ftp://example.com", "example.com", "javascript:alert(1)", "https://"} {
		d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
		_, err := d.svc.Update(context.Background(), 1, 3, service.UpdateDomainInput{RootRedirect: sp(bad)})
		assert.ErrorIs(t, err, service.ErrInvalidRedirectURL, bad)
	}
}

func TestDomainService_Update_Group(t *testing.T) {
	d := newDomainService(t)
	// Foreign group is rejected.
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
	d.groups.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.Group{ID: 9, UserID: 2}, nil)
	_, err := d.svc.Update(context.Background(), 1, 3, service.UpdateDomainInput{GroupID: u64(9)})
	assert.ErrorIs(t, err, service.ErrGroupNotFound)

	// Owned group is set.
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
	d.groups.EXPECT().GetByID(gomock.Any(), uint64(8)).Return(&model.Group{ID: 8, UserID: 1}, nil)
	saved := expectVerifyUpdate(d)
	got, err := d.svc.Update(context.Background(), 1, 3, service.UpdateDomainInput{GroupID: u64(8)})
	require.NoError(t, err)
	require.NotNil(t, got.GroupID)
	assert.Equal(t, uint64(8), *saved.GroupID)

	// ClearGroup removes it.
	withGroup := pendingDomain()
	withGroup.GroupID = u64(8)
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(withGroup, nil)
	expectVerifyUpdate(d)
	got, err = d.svc.Update(context.Background(), 1, 3, service.UpdateDomainInput{ClearGroup: true})
	require.NoError(t, err)
	assert.Nil(t, got.GroupID)
}

func TestDomainService_Delete(t *testing.T) {
	d := newDomainService(t)
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(pendingDomain(), nil)
	d.domains.EXPECT().Delete(gomock.Any(), uint64(3)).Return(nil)
	got, err := d.svc.Delete(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, "go.example.com", got.Domain)

	foreign := pendingDomain()
	foreign.UserID = 2
	d.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(foreign, nil)
	_, err = d.svc.Delete(context.Background(), 1, 3)
	assert.ErrorIs(t, err, service.ErrNotDomainOwner)
}

func TestDomainService_List(t *testing.T) {
	d := newDomainService(t)
	d.domains.EXPECT().ListByUserID(gomock.Any(), uint64(1)).Return(nil, nil)
	got, err := d.svc.List(context.Background(), 1)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
