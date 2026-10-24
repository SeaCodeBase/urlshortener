package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	repomocks "github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeDNS struct {
	txt   map[string][]string
	cname map[string]string
	// fail makes lookups of a name return this error (both TXT and CNAME).
	fail map[string]error
}

func (f *fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	if v, ok := f.txt[name]; ok {
		return v, nil
	}
	if err, ok := f.fail[name]; ok {
		return nil, err
	}
	return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

func (f *fakeDNS) LookupCNAME(_ context.Context, name string) (string, error) {
	if v, ok := f.cname[name]; ok {
		return v, nil
	}
	if err, ok := f.fail[name]; ok {
		return "", err
	}
	return "", &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

type domainRouter struct {
	r        *gin.Engine
	domains  *repomocks.MockDomainRepository
	groups   *repomocks.MockGroupRepository
	links    *repomocks.MockLinkRepository
	activity *mocks.MockActivityService
	dns      *fakeDNS
}

func newDomainRouter(t *testing.T) *domainRouter {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	dr := &domainRouter{
		domains:  repomocks.NewMockDomainRepository(ctrl),
		groups:   repomocks.NewMockGroupRepository(ctrl),
		links:    repomocks.NewMockLinkRepository(ctrl),
		activity: mocks.NewMockActivityService(ctrl),
		dns:      &fakeDNS{txt: map[string][]string{}, cname: map[string]string{}, fail: map[string]error{}},
	}
	svc := service.NewDomainService(dr.domains, dr.groups, dr.links, dr.dns, "sho.rt", "app.example.com")
	h := NewDomainHandler(svc, dr.activity)
	dr.r = gin.New()
	g := dr.r.Group("/api/domains", withGroup)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.PUT("/:id", h.Update)
	g.POST("/:id/verify", h.Verify)
	g.DELETE("/:id", h.Delete)
	return dr
}

const domainToken = "0123456789abcdef0123456789abcdef"

func handlerDomain() *model.Domain {
	return &model.Domain{ID: 3, UserID: 1, Domain: "go.example.com", VerificationToken: domainToken,
		VerificationStatus: model.DomainVerificationPending}
}

func TestDomainHandler_Verify_TXT(t *testing.T) {
	dr := newDomainRouter(t)
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(handlerDomain(), nil)
	dr.dns.txt["_urlshortener.go.example.com"] = []string{"verify=" + domainToken}
	dr.domains.EXPECT().UpdateVerification(gomock.Any(), uint64(3), model.DomainVerificationOK, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	dr.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDomainVerify, service.EntityDomain, uint64(3), gomock.Any())

	w := doJSON(dr.r, http.MethodPost, "/api/domains/3/verify", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["verification_status"])
	assert.Nil(t, body["verification_error"])
	assert.NotNil(t, body["verified_at"])
	assert.NotNil(t, body["last_checked_at"])
}

func TestDomainHandler_Verify_Failure200WithError(t *testing.T) {
	dr := newDomainRouter(t)
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(handlerDomain(), nil)
	dr.domains.EXPECT().UpdateVerification(gomock.Any(), uint64(3), model.DomainVerificationError, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	dr.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDomainVerify, service.EntityDomain, uint64(3), gomock.Any())

	w := doJSON(dr.r, http.MethodPost, "/api/domains/3/verify", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "error", body["verification_status"])
	assert.Contains(t, body["verification_error"], "TXT record not found")
}

func TestDomainHandler_Verify_Foreign404(t *testing.T) {
	dr := newDomainRouter(t)
	foreign := handlerDomain()
	foreign.UserID = 2
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(foreign, nil)
	w := doJSON(dr.r, http.MethodPost, "/api/domains/3/verify", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"domain not found"}`, w.Body.String())
}

func TestDomainHandler_List_FullFields(t *testing.T) {
	dr := newDomainRouter(t)
	dr.domains.EXPECT().ListByUserID(gomock.Any(), uint64(1)).Return([]*model.Domain{handlerDomain()}, nil)
	w := doJSON(dr.r, http.MethodGet, "/api/domains", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Domains []map[string]any `json:"domains"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Domains, 1)
	for _, k := range []string{"verification_status", "verification_token", "verification_error", "verified_at",
		"last_checked_at", "root_redirect", "not_found_redirect", "group_id"} {
		assert.Contains(t, body.Domains[0], k)
	}
}

func TestDomainHandler_List_Empty(t *testing.T) {
	dr := newDomainRouter(t)
	dr.domains.EXPECT().ListByUserID(gomock.Any(), uint64(1)).Return(nil, nil)
	w := doJSON(dr.r, http.MethodGet, "/api/domains", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"domains":[]}`, w.Body.String())
}

func TestDomainHandler_Create(t *testing.T) {
	dr := newDomainRouter(t)
	var created *model.Domain
	dr.domains.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d *model.Domain) error {
		d.ID = 3
		created = d
		return nil
	})
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).DoAndReturn(func(context.Context, uint64) (*model.Domain, error) {
		return created, nil
	})
	dr.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDomainCreate, service.EntityDomain, uint64(3), gomock.Any())

	w := doJSON(dr.r, http.MethodPost, "/api/domains", `{"domain":"Go.Example.com"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "go.example.com", body["domain"])
	assert.Equal(t, "pending", body["verification_status"])
	assert.Regexp(t, `^[0-9a-f]{32}$`, body["verification_token"])
}

func TestDomainHandler_Create_Errors(t *testing.T) {
	dr := newDomainRouter(t)
	w := doJSON(dr.r, http.MethodPost, "/api/domains", `{"domain":"https://go.example.com/x"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doJSON(dr.r, http.MethodPost, "/api/domains", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	dr.domains.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrDomainExists)
	taken := handlerDomain()
	taken.UserID = 2
	taken.VerificationStatus = model.DomainVerificationOK
	dr.domains.EXPECT().GetByDomain(gomock.Any(), "go.example.com").Return(taken, nil)
	w = doJSON(dr.r, http.MethodPost, "/api/domains", `{"domain":"go.example.com"}`)
	assert.Equal(t, http.StatusConflict, w.Code)

	w = doJSON(dr.r, http.MethodPost, "/api/domains", `{"domain":"sho.rt"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"this domain is reserved"}`, w.Body.String())

	dr.groups.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.Group{ID: 9, UserID: 2}, nil)
	w = doJSON(dr.r, http.MethodPost, "/api/domains", `{"domain":"go.example.com","group_id":9}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"group not found"}`, w.Body.String())
}

func TestDomainHandler_Update(t *testing.T) {
	dr := newDomainRouter(t)
	d := handlerDomain()
	d.GroupID = u64p(7)
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(d, nil)
	var saved model.Domain
	dr.domains.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d *model.Domain) error {
		saved = *d
		return nil
	})
	dr.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDomainUpdate, service.EntityDomain, uint64(3), gomock.Any())

	// group_id: null clears the group; root_redirect is set; not_found_redirect absent stays.
	w := doJSON(dr.r, http.MethodPut, "/api/domains/3", `{"root_redirect":"https://example.com","group_id":null}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Nil(t, saved.GroupID)
	require.NotNil(t, saved.RootRedirect)
	assert.Equal(t, "https://example.com", *saved.RootRedirect)
}

func TestDomainHandler_Update_InvalidRedirect400(t *testing.T) {
	dr := newDomainRouter(t)
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(handlerDomain(), nil)
	w := doJSON(dr.r, http.MethodPut, "/api/domains/3", `{"not_found_redirect":"ftp://x"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doJSON(dr.r, http.MethodPut, "/api/domains/3", `{"group_id":"abc"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDomainHandler_Delete(t *testing.T) {
	dr := newDomainRouter(t)
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(handlerDomain(), nil)
	dr.links.EXPECT().CountByDomainID(gomock.Any(), uint64(3)).Return(int64(0), nil)
	dr.domains.EXPECT().Delete(gomock.Any(), uint64(3)).Return(nil)
	dr.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDomainDelete, service.EntityDomain, uint64(3), gomock.Any())
	w := doJSON(dr.r, http.MethodDelete, "/api/domains/3", "")
	require.Equal(t, http.StatusOK, w.Code)

	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(4)).Return(nil, repository.ErrDomainNotFound)
	w = doJSON(dr.r, http.MethodDelete, "/api/domains/4", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDomainHandler_Delete_WithLinks409(t *testing.T) {
	dr := newDomainRouter(t)
	dr.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(handlerDomain(), nil)
	dr.links.EXPECT().CountByDomainID(gomock.Any(), uint64(3)).Return(int64(2), nil)
	w := doJSON(dr.r, http.MethodDelete, "/api/domains/3", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"This domain still has links. Delete or move them first."}`, w.Body.String())
}
