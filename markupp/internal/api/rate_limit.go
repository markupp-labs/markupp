package api

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AccountRateLimiter conta tentativas por chave numa janela fixa, em memória do processo
// (ADR-0044): com várias réplicas, o limite vale por réplica.
type AccountRateLimiter struct {
	limit     int
	window    time.Duration
	now       func() time.Time
	mu        sync.Mutex
	windows   map[string]attemptWindow
	lastSweep time.Time
	trusted   []netip.Prefix
}

type attemptWindow struct {
	start    time.Time
	attempts int
}

// NewAccountRateLimiter aceita limit tentativas por chave a cada window. Só conexões
// vindas de trusted têm o X-Forwarded-For lido para achar o IP do cliente.
func NewAccountRateLimiter(limit int, window time.Duration, now func() time.Time, trusted []netip.Prefix) *AccountRateLimiter {
	return &AccountRateLimiter{limit: limit, window: window, now: now, windows: map[string]attemptWindow{}, trusted: trusted}
}

// Allow registra uma tentativa de key e diz se ela cabe no limite. Recusada, diz
// quanto falta para a janela da chave acabar.
func (l *AccountRateLimiter) Allow(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweepExpired(now)
	current, ok := l.windows[key]
	if !ok || !now.Before(current.start.Add(l.window)) {
		current = attemptWindow{start: now}
	}
	if current.attempts >= l.limit {
		return current.start.Add(l.window).Sub(now), false
	}
	current.attempts++
	l.windows[key] = current
	return 0, true
}

// sweepExpired descarta as janelas vencidas, no máximo uma vez por janela, para a
// memória não crescer com IPs e emails que não voltam.
func (l *AccountRateLimiter) sweepExpired(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key, current := range l.windows {
		if !now.Before(current.start.Add(l.window)) {
			delete(l.windows, key)
		}
	}
}

// limitAccountAttempts limita a rota por IP e, quando o corpo traz email, também por
// email. Limitador nil deixa a rota livre.
func limitAccountAttempts(l *AccountRateLimiter) func(http.Handler) http.Handler {
	if l == nil {
		return passThrough
	}
	return limitRequests(l.allowRequest)
}

// limitRequests responde 429 ao pedido que allow recusa, com o tempo de espera.
func limitRequests(allow func(*http.Request) (time.Duration, bool)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if wait, ok := allow(r); !ok {
				writeRateLimited(w, wait)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// publicReadsKey prefixa a chave do limite da leitura pública, que é uma só por IP
// para todas as rotas, para o slug e o id da nota não abrirem limites novos.
const publicReadsKey = "public ip "

// limitPublicReads limita a leitura pública sem conta por IP (ADR-0048). Limitador nil
// deixa as rotas livres.
func limitPublicReads(l *AccountRateLimiter) func(http.Handler) http.Handler {
	if l == nil {
		return passThrough
	}
	return limitRequests(func(r *http.Request) (time.Duration, bool) {
		return l.Allow(publicReadsKey + l.clientIP(r))
	})
}

func passThrough(next http.Handler) http.Handler {
	return next
}

func (l *AccountRateLimiter) allowRequest(r *http.Request) (time.Duration, bool) {
	route := unversionedPath(r.URL.Path)
	keys := []string{route + " ip " + l.clientIP(r)}
	if email := peekEmail(r); email != "" {
		keys = append(keys, route+" email "+email)
	}
	for _, key := range keys {
		if wait, ok := l.Allow(key); !ok {
			return wait, false
		}
	}
	return 0, true
}

// clientIP usa o IP da conexão e, quando ela vem de proxy confiável, o primeiro IP
// não confiável do X-Forwarded-For lido da direita, porque o que está à esquerda
// dele o cliente escreve como quiser. Header vazio ou malformado fica com a conexão.
func (l *AccountRateLimiter) clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if addrPort, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		host = addrPort.Addr().String()
	}
	remote, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	return l.keyBehind(remote.Unmap(), forwardedHops(r))
}

// keyBehind é a chave do cliente atrás de remote, a conexão vista, com os saltos do
// X-Forwarded-For que chegaram nela.
func (l *AccountRateLimiter) keyBehind(remote netip.Addr, hops []string) string {
	if forwarded, ok := l.forwardedClient(hops, remote); ok {
		return ipKey(forwarded)
	}
	return ipKey(remote)
}

// forwardedHops junta os headers X-Forwarded-For numa lista de saltos, sempre com ao
// menos um item, vazio quando o header falta.
func forwardedHops(r *http.Request) []string {
	return strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
}

// forwardedClient percorre o X-Forwarded-For da direita para a esquerda enquanto o
// salto anterior é proxy confiável. Entrada que não é IP invalida o header.
func (l *AccountRateLimiter) forwardedClient(hops []string, remote netip.Addr) (netip.Addr, bool) {
	for i := len(hops) - 1; i >= 0 && l.isTrusted(remote); i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}, false
		}
		remote = hop.Unmap()
	}
	return remote, !l.isTrusted(remote)
}

func (l *AccountRateLimiter) isTrusted(addr netip.Addr) bool {
	for _, prefix := range l.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// ipKey agrupa IPv6 pelo prefixo /64, que costuma ser de um só cliente, senão cada
// endereço dele teria limite próprio.
func ipKey(addr netip.Addr) string {
	if addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr.WithZone(""), 64).Masked().String()
}

// peekEmail lê o email do corpo JSON e devolve o corpo inteiro para o handler.
func peekEmail(r *http.Request) string {
	head, err := io.ReadAll(io.LimitReader(r.Body, maxAuthBodyBytes))
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), r.Body), r.Body}
	if err != nil {
		return ""
	}
	var body struct {
		Email string `json:"email"`
	}
	// Decoder como o decodeJSONBody: lê o primeiro valor e ignora o resto, senão
	// lixo depois do JSON esconde o email do limite e o handler loga mesmo assim.
	if json.NewDecoder(bytes.NewReader(head)).Decode(&body) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(body.Email))
}

func writeRateLimited(w http.ResponseWriter, wait time.Duration) {
	seconds := max(1, int(math.Ceil(wait.Seconds())))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, "rate_limited", "tentativas demais, aguarde "+strconv.Itoa(seconds)+"s", http.StatusTooManyRequests)
}

// unversionedPath tira o prefixo /v1 do caminho, para a rota sob /v1 e o alias obsoleto
// sem prefixo contarem no mesmo limite.
func unversionedPath(path string) string {
	if rest, ok := strings.CutPrefix(path, currentVersionPrefix+"/"); ok {
		return "/" + rest
	}
	return path
}
