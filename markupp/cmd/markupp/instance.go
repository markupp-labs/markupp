package main

import (
	"context"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/secretbox"
)

// prepareInstance completa a config com os segredos do banco (ADR-0041) e escreve nos
// volumes, quando há caminho, o do servidor MCP da mesma origem e o de relay do painel.
func prepareInstance(ctx context.Context, cfg config.Config, secrets config.InstanceSecrets) (config.Config, error) {
	prepared, err := cfg.WithInstanceSecrets(ctx, secrets, auth.NewOpaqueToken)
	if err != nil {
		return config.Config{}, err
	}
	if secret, sameOrigin := prepared.DefaultMCPResourceSecret(); sameOrigin {
		err = writeSecretIfPath(prepared.MCPSecretPath, secret)
	}
	if err == nil {
		err = writeSecretIfPath(prepared.PanelRelaySecretPath, prepared.PanelRelaySecret)
	}
	if err != nil {
		return config.Config{}, err
	}
	return prepared, nil
}

// writeSecretIfPath escreve o segredo no arquivo, quando quem hospeda deu o caminho.
func writeSecretIfPath(path, secret string) error {
	if path == "" {
		return nil
	}
	return config.WriteSecretFile(path, secret)
}

// secretKeyName é o segredo do banco que cifra os outros quando quem hospeda não define
// MARKUPP_SECRET_KEY (ADR-0057).
const secretKeyName = "secret_key"

// newSecretBox monta a cifra dos segredos guardados no banco. A chave ativa é secretKey, a
// MARKUPP_SECRET_KEY, e o segredo da instância no banco, gerado na primeira vez, fica de
// chave antiga, para abrir o que foi selado antes da variável. Sem a variável, ele é a
// ativa, e quem tem uma cópia do banco também decifra.
func newSecretBox(ctx context.Context, secretKey string, secrets config.InstanceSecrets) (*secretbox.Keyring, error) {
	stored, err := secrets.Secret(ctx, secretKeyName, auth.NewOpaqueToken)
	if err != nil {
		return nil, err
	}
	if secretKey == "" {
		return secretbox.NewKeyring(stored)
	}
	return secretbox.NewKeyring(secretKey, stored)
}
