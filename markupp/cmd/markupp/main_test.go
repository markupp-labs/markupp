package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewServer_PortaConfigurada_DefineEnderecoETimeouts(t *testing.T) {
	server := newServer(8080, http.NotFoundHandler())

	assert.Equal(t, ":8080", server.Addr)
	assert.Greater(t, server.ReadHeaderTimeout, time.Duration(0),
		"ReadHeaderTimeout zerado deixa o servidor exposto a slowloris")
	assert.Greater(t, server.ReadTimeout, time.Duration(0))
	assert.Greater(t, server.WriteTimeout, time.Duration(0))
	assert.Greater(t, server.IdleTimeout, time.Duration(0))
}
