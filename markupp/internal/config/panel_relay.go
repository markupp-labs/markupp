package config

import (
	"context"
	"fmt"
)

const panelRelaySecretName = "panel_relay_secret" // #nosec G101 -- nome da linha no banco, nao o segredo

// loadPanelRelaySettings lê o segredo com que o painel se apresenta como relay da leitura
// pública e o arquivo onde o servidor o escreve para o painel ler.
func loadPanelRelaySettings(lookup LookupFunc, cfg *Config) error {
	readString(lookup, "MARKUPP_PANEL_RELAY_SECRET_PATH", &cfg.PanelRelaySecretPath)
	return readSecret(lookup, "MARKUPP_PANEL_RELAY_SECRET", &cfg.PanelRelaySecret)
}

// withPanelRelaySecret completa, sem a variável, o segredo de relay do painel com o do
// banco, gerado na primeira subida como o do MCP.
func (c Config) withPanelRelaySecret(ctx context.Context, secrets InstanceSecrets, generate func() (string, error)) (Config, error) {
	if c.PanelRelaySecret != "" {
		return c, nil
	}
	secret, err := secrets.Secret(ctx, panelRelaySecretName, generate)
	if err != nil {
		return Config{}, fmt.Errorf("read instance secret %s: %w", panelRelaySecretName, err)
	}
	c.PanelRelaySecret = secret
	return c, nil
}
