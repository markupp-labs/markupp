package cimd

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

// DefaultTTL vale quando a resposta não traz max-age.
const DefaultTTL = time.Hour

// cacheTTL lê o Cache-Control e limita o resultado a [minTTL, maxTTL].
func cacheTTL(header string, minTTL, maxTTL time.Duration) time.Duration {
	return max(minTTL, min(rawCacheTTL(header), maxTTL))
}

// rawCacheTTL devolve zero para no-store, no-cache ou max-age ilegível: o limite
// inferior entra no lugar, na leitura mais conservadora.
func rawCacheTTL(header string) time.Duration {
	ttl := DefaultTTL
	for _, directive := range strings.Split(strings.ToLower(header), ",") {
		directive = strings.TrimSpace(directive)
		switch {
		case directive == "no-store" || directive == "no-cache":
			return 0
		case strings.HasPrefix(directive, "max-age="):
			ttl = parseMaxAge(strings.TrimPrefix(directive, "max-age="))
		}
	}
	return ttl
}

func parseMaxAge(value string) time.Duration {
	// Em ErrRange o ParseInt já devolve o extremo do int64, que o clamp absorve.
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0
	}
	return time.Duration(min(max(seconds, 0), math.MaxInt64/int64(time.Second))) * time.Second
}
