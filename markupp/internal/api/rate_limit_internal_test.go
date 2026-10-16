package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAccountRateLimiter_EntradaDeJanelaVencida_EhDescartada(t *testing.T) {
	current := time.Unix(1_000_000, 0)
	limiter := NewAccountRateLimiter(5, time.Minute, func() time.Time { return current })
	limiter.Allow("ip 10.0.0.1")
	limiter.Allow("ip 10.0.0.2")
	current = current.Add(time.Minute)

	limiter.Allow("ip 10.0.0.3")

	assert.Len(t, limiter.windows, 1, "so a chave da janela corrente fica na memoria")
}
