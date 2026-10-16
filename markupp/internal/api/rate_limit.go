package api

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AccountRateLimiter conta tentativas por chave numa janela fixa, em memória do processo
// (ADR-0037): com várias réplicas, o limite vale por réplica.
type AccountRateLimiter struct {
	limit     int
	window    time.Duration
	now       func() time.Time
	mu        sync.Mutex
	windows   map[string]attemptWindow
	lastSweep time.Time
}

type attemptWindow struct {
	start    time.Time
	attempts int
}

// NewAccountRateLimiter aceita limit tentativas por chave a cada window.
func NewAccountRateLimiter(limit int, window time.Duration, now func() time.Time) *AccountRateLimiter {
	return &AccountRateLimiter{limit: limit, window: window, now: now, windows: map[string]attemptWindow{}}
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
	return func(next http.Handler) http.Handler {
		if l == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if wait, ok := l.allowRequest(r); !ok {
				writeRateLimited(w, wait)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *AccountRateLimiter) allowRequest(r *http.Request) (time.Duration, bool) {
	keys := []string{r.URL.Path + " ip " + clientIP(r)}
	if email := peekEmail(r); email != "" {
		keys = append(keys, r.URL.Path+" email "+email)
	}
	for _, key := range keys {
		if wait, ok := l.Allow(key); !ok {
			return wait, false
		}
	}
	return 0, true
}

// clientIP usa só a conexão: sem proxy confiável configurado, X-Forwarded-For é do
// cliente e não serve de chave. IPv6 vira o prefixo /64, que costuma ser de um só
// cliente, senão cada endereço dele teria limite próprio.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() != nil {
		return host
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
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
