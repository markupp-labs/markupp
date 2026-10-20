package config

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	authSecretName        = "auth_secret"
	mcpResourceSecretName = "mcp_resource_secret" // #nosec G101 -- nome da linha no banco, nao o segredo
	mcpUpstreamExpected   = "http(s)://host[:porta] do servidor MCP, sem caminho, query nem fragmento"
)

// InstanceSecrets guarda os segredos que o servidor gera na primeira subida (ADR-0041).
type InstanceSecrets interface {
	Secret(ctx context.Context, name string, generate func() (string, error)) (string, error)
}

// loadMCPSettings lê o servidor MCP da mesma origem: para onde /mcp vai e onde o
// segredo dele é escrito.
func loadMCPSettings(lookup LookupFunc, cfg *Config) error {
	readString(lookup, "MARKUPP_MCP_SECRET_PATH", &cfg.MCPSecretPath)
	var upstream string
	readString(lookup, "MARKUPP_MCP_UPSTREAM_URL", &upstream)
	if upstream == "" {
		return nil
	}
	parsed, err := url.Parse(upstream)
	if err != nil || !mcpUpstreamAllowed(parsed) {
		return fmt.Errorf("MARKUPP_MCP_UPSTREAM_URL=%q, esperado %s", upstream, mcpUpstreamExpected)
	}
	cfg.MCPUpstreamURL = strings.TrimRight(upstream, "/")
	return nil
}

// mcpUpstreamAllowed aceita http em qualquer host: é o endereço interno do container.
func mcpUpstreamAllowed(parsed *url.URL) bool {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	noExtras := parsed.RawQuery == "" && parsed.Fragment == "" && parsed.User == nil
	return parsed.Hostname() != "" && noExtras && strings.Trim(parsed.Path, "/") == ""
}

// DefaultMCPResourceURL é o servidor MCP na mesma origem do issuer.
func (c Config) DefaultMCPResourceURL() string {
	return c.PublicURL + "/mcp"
}

// DefaultMCPResourceSecret devolve o segredo do MCP da mesma origem, quando ele é um dos
// resources; o MCP em subdomínio recebe o segredo por fora.
func (c Config) DefaultMCPResourceSecret() (string, bool) {
	for _, resource := range c.OAuthResources {
		if resource.URL == c.DefaultMCPResourceURL() {
			return resource.Secret, true
		}
	}
	return "", false
}

// WithInstanceSecrets completa o que as variáveis deixaram vazio com os segredos do
// banco: o do JWT e, sem MARKUPP_OAUTH_RESOURCES, o do MCP da mesma origem.
func (c Config) WithInstanceSecrets(ctx context.Context, secrets InstanceSecrets, generate func() (string, error)) (Config, error) {
	if c.AuthSecret == "" {
		secret, err := secrets.Secret(ctx, authSecretName, generate)
		if err != nil {
			return Config{}, fmt.Errorf("segredo %s da instancia: %w", authSecretName, err)
		}
		c.AuthSecret = secret
	}
	if len(c.OAuthResources) > 0 {
		return c, nil
	}
	secret, err := secrets.Secret(ctx, mcpResourceSecretName, generate)
	if err != nil {
		return Config{}, fmt.Errorf("segredo %s da instancia: %w", mcpResourceSecretName, err)
	}
	c.OAuthResources = []OAuthResource{{URL: c.DefaultMCPResourceURL(), Secret: secret}}
	return c, nil
}

// WriteMCPSecret escreve o segredo no volume que o servidor MCP lê, por arquivo temporário
// e rename, para o MCP nunca ler o arquivo pela metade. O erro cita o caminho, nunca o
// segredo.
func WriteMCPSecret(path, secret string) error {
	dir := filepath.Dir(path)
	// #nosec G301 -- o servidor MCP roda com outro usuario e precisa atravessar o diretorio
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("criar o diretorio %s do segredo do MCP: %w", dir, err)
	}
	temp, err := os.CreateTemp(dir, ".secret-*")
	if err != nil {
		return fmt.Errorf("criar arquivo temporario em %s para o segredo do MCP: %w", dir, err)
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if err := writeAndClose(temp, secret); err != nil {
		return fmt.Errorf("escrever o segredo do MCP em %s: %w", temp.Name(), err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("mover o segredo do MCP para %s: %w", path, err)
	}
	return nil
}

// writeAndClose grava o segredo legível pelo usuário do container do MCP, que não é o
// do servidor; o volume só é montado nos dois.
func writeAndClose(file *os.File, secret string) error {
	if _, err := file.WriteString(secret); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
