package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

type fakeIPBlockService struct {
	blocks  []instance.IPBlock
	created instance.IPBlock
	err     error

	gotRequest            instance.IPBlockRequest
	gotActor, gotDeleteID string
}

func (f *fakeIPBlockService) List(context.Context) ([]instance.IPBlock, error) {
	return f.blocks, f.err
}

func (f *fakeIPBlockService) Create(_ context.Context, req instance.IPBlockRequest) (instance.IPBlock, error) {
	f.gotRequest = req
	return f.created, f.err
}

func (f *fakeIPBlockService) Delete(_ context.Context, actorID, id string) error {
	f.gotActor, f.gotDeleteID = actorID, id
	return f.err
}

// fakeBlockedIPs bloqueia só os endereços dados.
type fakeBlockedIPs map[netip.Addr]bool

func (f fakeBlockedIPs) Blocked(addr netip.Addr) bool { return f[addr] }

func ipBlocksRequest(svc *fakeIPBlockService, blocked fakeBlockedIPs, method, path, body, remoteAddr string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: administradorDaInstancia}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		InstanceAdmins: instance.Admins{emailDoAdministrador}, IPBlocks: svc, BlockedIPs: blocked, Probe: &fakeProbe{},
	})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

const ipDoAdministrador = "203.0.113.7:51000"

func TestIPBlocks_Listar(t *testing.T) {
	svc := &fakeIPBlockService{blocks: []instance.IPBlock{{ID: "b1", Prefix: netip.MustParsePrefix("198.51.100.0/24"),
		Note: "varredura", CreatedAt: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)}}}

	rec := ipBlocksRequest(svc, nil, http.MethodGet, "/v1/instance/ip-blocks", "", ipDoAdministrador)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"b1","cidr":"198.51.100.0/24","note":"varredura","created_at":"2026-11-25T12:00:00Z"}]`, rec.Body.String())
}

func TestIPBlocks_Criar_PassaOIPDoAdministradorEResponde201(t *testing.T) {
	svc := &fakeIPBlockService{created: instance.IPBlock{ID: "b1", Prefix: netip.MustParsePrefix("198.51.100.0/24")}}

	rec := ipBlocksRequest(svc, nil, http.MethodPost, "/v1/instance/ip-blocks", `{"cidr":"198.51.100.0/24","note":"x"}`, ipDoAdministrador)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, instance.IPBlockRequest{ActorID: "u1", ActorIP: "203.0.113.7", CIDR: "198.51.100.0/24", Note: "x"}, svc.gotRequest)
	assert.Contains(t, rec.Body.String(), `"cidr":"198.51.100.0/24"`)
}

func TestIPBlocks_ErrosViramRespostas(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: %q", instance.ErrInvalidIPBlock, "x"), http.StatusBadRequest, "invalid_ip_block"},
		{instance.ErrBlocksOwnIP, http.StatusConflict, "blocks_own_ip"},
		{instance.ErrIPBlockExists, http.StatusConflict, "ip_block_exists"},
	}
	for _, c := range casos {
		rec := ipBlocksRequest(&fakeIPBlockService{err: c.err}, nil, http.MethodPost, "/v1/instance/ip-blocks", `{"cidr":"x"}`, ipDoAdministrador)

		assert.Equal(t, c.status, rec.Code, c.code)
		assert.Contains(t, rec.Body.String(), `"error":"`+c.code+`"`)
	}
}

func TestIPBlocks_Apagar(t *testing.T) {
	svc := &fakeIPBlockService{}

	rec := ipBlocksRequest(svc, nil, http.MethodDelete, "/v1/instance/ip-blocks/b1", "", ipDoAdministrador)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "u1", svc.gotActor)
	assert.Equal(t, "b1", svc.gotDeleteID)
}

func TestIPBlocks_ApagarInexistente_404(t *testing.T) {
	rec := ipBlocksRequest(&fakeIPBlockService{err: instance.ErrIPBlockNotFound}, nil, http.MethodDelete, "/v1/instance/ip-blocks/b1", "", ipDoAdministrador)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"ip_block_not_found"`)
}

func TestIPBlocks_IPBloqueado_403EmTodaRotaMenosASonda(t *testing.T) {
	bloqueado := fakeBlockedIPs{netip.MustParseAddr("198.51.100.9"): true}

	for _, path := range []string{"/v1/auth/login", "/oauth/token", "/v1/instance/ip-blocks", "/mcp"} {
		rec := ipBlocksRequest(&fakeIPBlockService{}, bloqueado, http.MethodPost, path, `{}`, "198.51.100.9:51000")

		assert.Equal(t, http.StatusForbidden, rec.Code, path)
		assert.Contains(t, rec.Body.String(), `"error":"ip_blocked"`, path)
	}
	sonda := ipBlocksRequest(&fakeIPBlockService{}, bloqueado, http.MethodGet, "/v1/healthz", "", "198.51.100.9:51000")
	assert.Equal(t, http.StatusOK, sonda.Code)
}

func TestIPBlocks_IPLivre_Passa(t *testing.T) {
	bloqueado := fakeBlockedIPs{netip.MustParseAddr("198.51.100.9"): true}

	rec := ipBlocksRequest(&fakeIPBlockService{}, bloqueado, http.MethodGet, "/v1/instance/ip-blocks", "", ipDoAdministrador)

	assert.Equal(t, http.StatusOK, rec.Code)
}
