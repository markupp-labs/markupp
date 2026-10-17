package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/netip"
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
		AccountRateLimiter: api.NewAccountRateLimiter(limiteDeTeste, time.Minute, clock.Now, nil),
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
		AccountRateLimiter: api.NewAccountRateLimiter(limiteDeTeste, time.Minute, time.Now, nil),
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

func TestRateLimit_MesmoEmailComLixoDepoisDoJSON_Responde429(t *testing.T) {
	router := newLimitedRouter(&fakeRateClock{current: time.Unix(1_000_000, 0)})
	corpo := `{"email":"ana@markupp.dev","password":"x"}x`
	for i := 0; i < limiteDeTeste; i++ {
		limitedPost(router, "/auth/login", "10.0.0."+strconv.Itoa(i+1)+":1234", corpo)
	}

	rec := limitedPost(router, "/auth/login", "10.0.0.99:1234", corpo)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimit_IPv6DoMesmoPrefixo64_DividemOLimite(t *testing.T) {
	router := newLimitedRouter(&fakeRateClock{current: time.Unix(1_000_000, 0)})
	esgotaLimite(t, router, "/auth/refresh", "[2001:db8:0:1::1]:1234", `{}`)

	rec := limitedPost(router, "/auth/refresh", "[2001:db8:0:1:ffff::2]:1234", `{}`)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimit_IPv6DeOutroPrefixo64_TemLimiteProprio(t *testing.T) {
	router := newLimitedRouter(&fakeRateClock{current: time.Unix(1_000_000, 0)})
	esgotaLimite(t, router, "/auth/refresh", "[2001:db8:0:1::1]:1234", `{}`)

	rec := limitedPost(router, "/auth/refresh", "[2001:db8:0:2::1]:1234", `{}`)

	assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
}

var proxiesDeTeste = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("fd00::/8")}

func newRouterAtrasDeProxy() http.Handler {
	clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
	return api.NewRouter(api.RouterDeps{
		Auth:               &fakeAuthService{},
		Tokens:             &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		AccountRateLimiter: api.NewAccountRateLimiter(limiteDeTeste, time.Minute, clock.Now, proxiesDeTeste),
	})
}

func refreshPeloProxy(router http.Handler, remoteAddr, forwardedFor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(`{}`))
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func esgotaPeloProxy(t *testing.T, router http.Handler, remoteAddr string, forwardedFor func(int) string) {
	t.Helper()
	for i := 0; i < limiteDeTeste; i++ {
		rec := refreshPeloProxy(router, remoteAddr, forwardedFor(i))
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "tentativa %d", i+1)
	}
}

func TestRateLimit_ProxyConfiavel_DaLimiteProprioACadaClienteDoHeader(t *testing.T) {
	router := newRouterAtrasDeProxy()
	esgotaPeloProxy(t, router, "10.0.0.1:443", func(int) string { return "198.51.100.1" })

	rec := refreshPeloProxy(router, "10.0.0.1:443", "198.51.100.2")

	assert.NotEqual(t, http.StatusTooManyRequests, rec.Code, "outro cliente atras do mesmo proxy")
}

func TestRateLimit_HeaderDeOrigemNaoConfiavel_EhIgnorado(t *testing.T) {
	router := newRouterAtrasDeProxy()
	esgotaPeloProxy(t, router, "203.0.113.9:1234", func(i int) string { return "198.51.100." + strconv.Itoa(i+1) })

	rec := refreshPeloProxy(router, "203.0.113.9:1234", "198.51.100.99")

	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "cliente fora dos proxies nao escolhe o proprio IP")
}

func TestRateLimit_CadeiaDeProxies_UsaOPrimeiroIPNaoConfiavelDaDireita(t *testing.T) {
	router := newRouterAtrasDeProxy()
	esgotaPeloProxy(t, router, "10.0.0.1:443", func(i int) string {
		return "192.0.2." + strconv.Itoa(i+1) + ", 198.51.100.7, 10.0.0.2"
	})

	rec := refreshPeloProxy(router, "10.0.0.3:443", "198.51.100.7")

	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "o que vem antes do cliente no header e forjavel")
}

func TestRateLimit_ProxyConfiavelSemHeaderOuComLixo_UsaOIPDaConexao(t *testing.T) {
	for _, header := range []string{"lixo", "198.51.100.1, lixo", "198.51.100.1:5678", " , "} {
		router := newRouterAtrasDeProxy()
		esgotaPeloProxy(t, router, "10.0.0.1:443", func(int) string { return "" })

		rec := refreshPeloProxy(router, "10.0.0.1:443", header)

		assert.Equal(t, http.StatusTooManyRequests, rec.Code, header)
	}
}

func TestRateLimit_ProxyConfiavel_AgrupaIPv6DoHeaderPeloPrefixo64(t *testing.T) {
	router := newRouterAtrasDeProxy()
	esgotaPeloProxy(t, router, "[fd00::1]:443", func(int) string { return "2001:db8:0:1::1" })

	rec := refreshPeloProxy(router, "[fd00::1]:443", "2001:db8:0:1:ffff::2")

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
