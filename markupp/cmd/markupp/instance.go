package main

import (
	"context"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
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
