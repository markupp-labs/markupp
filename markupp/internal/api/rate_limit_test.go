package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

const limiteDeTeste = 5

// fakeRateClock faz o papel do relógio do limitador, avançado à mão pelo teste.
type fakeRateClock struct {
	current time.Time
}

func (c *fakeRateClock) Now() time.Time { return c.current }

func newLimitedRouter(clock *fakeRateClock) http.Handler {
	return api.NewRouter(api.RouterDeps{
		Auth:               &fakeAuthService{},
		AccountEmail:       &fakeAccountEmailService{},
		Tokens:             &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		AccountRateLimiter: api.NewAccountRateLimiter(limiteDeTeste, time.Minute, clock.Now),
	})
}

func limitedPost(router http.Handler, path, remoteAddr, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func esgotaLimite(t *testing.T, router http.Handler, path, remoteAddr, body string) {
	t.Helper()
	for i := 0; i < limiteDeTeste; i++ {
		rec := limitedPost(router, path, remoteAddr, body)
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "tentativa %d de %s", i+1, path)
	}
}

func TestRateLimit_SextaTentativaDoMesmoIP_Responde429ComRetryAfter(t *testing.T) {
	rotas := []string{"/auth/login", "/auth/password/forgot", "/auth/password/reset", "/auth/email/verification", "/auth/invites/accept", "/auth/refresh"}
	for _, rota := range rotas {
		clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
		router := newLimitedRouter(clock)
		esgotaLimite(t, router, rota, "10.0.0.1:1234", `{}`)
		clock.current = clock.current.Add(20 * time.Second)

		rec := limitedPost(router, rota, "10.0.0.1:1234", `{}`)

		assert.Equal(t, http.StatusTooManyRequests, rec.Code, rota)
		assert.Equal(t, "rate_limited", codigoDoErro(t, rec), rota)
		assert.Equal(t, "40", rec.Header().Get("Retry-After"), rota)
	}
}

func TestRateLimit_OutroIP_TemLimiteProprio(t *testing.T) {
	router := newLimitedRouter(&fakeRateClock{current: time.Unix(1_000_000, 0)})
	esgotaLimite(t, router, "/auth/refresh", "10.0.0.1:1234", `{}`)

	rec := limitedPost(router, "/auth/refresh", "10.0.0.2:1234", `{}`)

	assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimit_MesmoIPOutraPorta_ContaComoOMesmoIP(t *testing.T) {
	router := newLimitedRouter(&fakeRateClock{current: time.Unix(1_000_000, 0)})
	esgotaLimite(t, router, "/auth/refresh", "10.0.0.1:1234", `{}`)

	rec := limitedPost(router, "/auth/refresh", "10.0.0.1:5678", `{}`)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimit_MesmoEmailDeIPsDiferentes_Responde429(t *testing.T) {
	router := newLimitedRouter(&fakeRateClock{current: time.Unix(1_000_000, 0)})
	for i := 0; i < limiteDeTeste; i++ {
		limitedPost(router, "/auth/login", "10.0.0."+strconv.Itoa(i+1)+":1234", `{"email":"Ana@Markupp.dev","password":"x"}`)
	}

	rec := limitedPost(router, "/auth/login", "10.0.0.99:1234", `{"email":"ana@markupp.dev","password":"x"}`)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimit_JanelaPassada_LiberaDeNovo(t *testing.T) {
	clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
	router := newLimitedRouter(clock)
	esgotaLimite(t, router, "/auth/login", "10.0.0.1:1234", `{"email":"ana@markupp.dev"}`)
	clock.current = clock.current.Add(time.Minute)

	rec := limitedPost(router, "/auth/login", "10.0.0.1:1234", `{"email":"ana@markupp.dev"}`)

	assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimit_RotaLimitada_EntregaOCorpoIntactoAoHandler(t *testing.T) {
	svc := &fakeAccountEmailService{}
	router := api.NewRouter(api.RouterDeps{
		AccountEmail:       svc,
		Tokens:             &fakeTokenValidator{},
		AccountRateLimiter: api.NewAccountRateLimiter(limiteDeTeste, time.Minute, time.Now),
	})

	rec := limitedPost(router, "/auth/password/forgot", "10.0.0.1:1234", `{"email":"ana@markupp.dev"}`)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "ana@markupp.dev", svc.resetEmail)
}

func TestRateLimit_RotaForaDaLista_NaoLimita(t *testing.T) {
	router := newLimitedRouter(&fakeRateClock{current: time.Unix(1_000_000, 0)})
	for i := 0; i < limiteDeTeste+1; i++ {
		rec := limitedPost(router, "/auth/logout", "10.0.0.1:1234", `{}`)

		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	}
}
