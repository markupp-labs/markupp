package main

import (
	"context"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/secretbox"
)

// prepareInstance completa a config com os segredos do banco (ADR-0041) e escreve o do
// servidor MCP da mesma origem no volume que ele lê, quando há caminho para isso.
func prepareInstance(ctx context.Context, cfg config.Config, secrets config.InstanceSecrets) (config.Config, error) {
	prepared, err := cfg.WithInstanceSecrets(ctx, secrets, auth.NewOpaqueToken)
	if err != nil {
		return config.Config{}, err
	}
	secret, sameOrigin := prepared.DefaultMCPResourceSecret()
	if prepared.MCPSecretPath == "" || !sameOrigin {
		return prepared, nil
	}
	if err := config.WriteMCPSecret(prepared.MCPSecretPath, secret); err != nil {
		return config.Config{}, err
	}
	return prepared, nil
}

// secretKeyName é o segredo do banco que cifra os outros quando quem hospeda não define
// MARKUPP_SECRET_KEY (ADR-0057).
const secretKeyName = "secret_key"

// newSecretBox monta a cifra dos segredos guardados no banco. A chave vem de secretKey,
// a MARKUPP_SECRET_KEY, e sem ela do segredo da instância no banco, gerado na primeira
// vez: aí quem tem uma cópia do banco também decifra.
func newSecretBox(ctx context.Context, secretKey string, secrets config.InstanceSecrets) (*secretbox.Box, error) {
	if secretKey == "" {
		var err error
		if secretKey, err = secrets.Secret(ctx, secretKeyName, auth.NewOpaqueToken); err != nil {
			return nil, err
		}
	}
	return secretbox.New(secretKey)
}
