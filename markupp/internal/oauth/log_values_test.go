package oauth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// longClientValue é um valor vindo do cliente, longo e com caractere de controle.
var longClientValue = "\x01" + strings.Repeat("x", 300)

func TestErroEmbrulhado_ValorDoCliente_ChegaLimitadoAoLog(t *testing.T) {
	casos := map[string]func() error{
		"falha ao buscar o cliente": func() error {
			repo := testRepositoryWithClient()
			repo.getClientErr = errors.New("banco fora do ar")
			params := validAuthorizeParams()
			params.ClientID = longClientValue
			_, err := newTestService(repo).StartAuthorization(context.Background(), params)
			return err
		},
		"falha ao criar o pedido": func() error {
			repo := newFakeOAuthRepository()
			repo.clients[longClientValue] = oauth.Client{ID: longClientValue, RedirectURIs: []string{testRedirectURI}}
			repo.createErr = errors.New("banco fora do ar")
			params := validAuthorizeParams()
			params.ClientID = longClientValue
			_, err := newTestService(repo).StartAuthorization(context.Background(), params)
			return err
		},
		"pedido inexistente ao descrever": func() error {
			_, err := newTestService(newFakeOAuthRepository()).DescribeRequest(context.Background(), longClientValue, testUserID)
			return err
		},
		"cliente apagado ao descrever": func() error {
			repo := newFakeOAuthRepository()
			repo.requests[longClientValue] = pendingTestRequest()
			_, err := newTestService(repo).DescribeRequest(context.Background(), longClientValue, testUserID)
			return err
		},
		"pedido inexistente ao aprovar": func() error {
			_, err := newTestService(newFakeOAuthRepository()).Approve(context.Background(), longClientValue, testUserID)
			return err
		},
		"pedido inexistente ao recusar": func() error {
			_, err := newTestService(newFakeOAuthRepository()).Deny(context.Background(), longClientValue, testUserID)
			return err
		},
	}
	for nome, falha := range casos {
		t.Run(nome, func(t *testing.T) {
			err := falha()

			require.Error(t, err)
			assert.NotContains(t, err.Error(), longClientValue[1:], "o valor do cliente deve chegar truncado")
			assert.NotContains(t, err.Error(), "\x01")
			assert.NotContains(t, err.Error(), `\x01`)
		})
	}
}
