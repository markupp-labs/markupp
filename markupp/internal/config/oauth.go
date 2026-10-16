package config

import (
	"fmt"
	"net/url"
	"strings"
)

// OAuthResource é um servidor de recursos que aceita os tokens do authorization
// server: URL é o identificador do recurso e Secret o segredo que ele apresenta.
type OAuthResource struct {
	URL    string
	Secret string
}

const minOAuthResourceSecretBytes = 32

// oauthURLRule descreve o que cada variável de URL do OAuth aceita além do esquema.
type oauthURLRule struct {
	key       string
	expected  string
	allowPath bool
}

var (
	publicURLRule = oauthURLRule{key: "MARKUPP_PUBLIC_URL",
		expected: "https://host[:porta] sem caminho, query nem fragmento (http so em localhost, 127.0.0.1 ou [::1])"}
	panelURLRule = oauthURLRule{key: "MARKUPP_PANEL_URL", allowPath: true,
		expected: "https://host[:porta][/caminho] sem query nem fragmento (http so em localhost, 127.0.0.1 ou [::1])"}
	resourceURLRule = oauthURLRule{key: "MARKUPP_OAUTH_RESOURCES", allowPath: true,
		expected: "url=segredo com url https://host[:porta][/caminho] sem query nem fragmento (http so em localhost, 127.0.0.1 ou [::1])"}
)

// OAuthEnabled diz se o authorization server está ligado, o que MARKUPP_PUBLIC_URL decide.
func (c Config) OAuthEnabled() bool {
	return c.PublicURL != ""
}

// loadOAuthSettings lê o authorization server (ADR-0038). Sem MARKUPP_PUBLIC_URL ele
// fica desligado e as demais variáveis nem são lidas.
func loadOAuthSettings(lookup LookupFunc, cfg *Config) error {
	var public string
	readString(lookup, publicURLRule.key, &public)
	if public == "" {
		return nil
	}
	if err := checkOAuthURL(public, publicURLRule); err != nil {
		return err
	}
	cfg.PublicURL = strings.TrimRight(public, "/")
	if err := loadPanelURL(lookup, cfg); err != nil {
		return err
	}
	var err error
	cfg.OAuthResources, err = readOAuthResources(lookup)
	return err
}

// loadPanelURL exige o painel, para onde o authorization server manda o login e o consentimento.
func loadPanelURL(lookup LookupFunc, cfg *Config) error {
	var panel string
	readString(lookup, panelURLRule.key, &panel)
	if panel == "" {
		return fmt.Errorf("MARKUPP_PANEL_URL vazia, esperado %s quando MARKUPP_PUBLIC_URL existe", panelURLRule.expected)
	}
	if err := checkOAuthURL(panel, panelURLRule); err != nil {
		return err
	}
	cfg.PanelURL = strings.TrimRight(panel, "/")
	return nil
}

// checkOAuthURL valida raw pela regra. A mensagem de erro mostra a URL com a senha mascarada.
func checkOAuthURL(raw string, rule oauthURLRule) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s=%q, esperado %s", rule.key, "valor ilegivel", rule.expected)
	}
	if !oauthURLAllowed(parsed, rule) || strings.Contains(raw, "#") {
		return fmt.Errorf("%s=%q, esperado %s", rule.key, parsed.Redacted(), rule.expected)
	}
	return nil
}

// oauthURLAllowed confere esquema, host e ausência de userinfo e de query e, conforme
// a regra, o caminho.
func oauthURLAllowed(parsed *url.URL, rule oauthURLRule) bool {
	if !oauthSchemeAllowed(parsed) || parsed.User != nil {
		return false
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return false
	}
	return rule.allowPath || parsed.Path == "" || parsed.Path == "/"
}

// oauthSchemeAllowed aceita https com host e http só em loopback. Hostname
// descarta a porta, então https://:8443 cai como host vazio.
func oauthSchemeAllowed(parsed *url.URL) bool {
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	return parsed.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1")
}

// readOAuthResources lê os pares url=segredo separados por vírgula, exigindo ao
// menos um e sem URL repetida.
func readOAuthResources(lookup LookupFunc) ([]OAuthResource, error) {
	raw, _ := lookup(resourceURLRule.key)
	var resources []OAuthResource
	for _, entry := range strings.Split(raw, ",") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		resource, err := parseOAuthResource(strings.TrimSpace(entry), len(resources)+1)
		if err != nil {
			return nil, err
		}
		if resources, err = appendUniqueResource(resources, resource); err != nil {
			return nil, err
		}
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("MARKUPP_OAUTH_RESOURCES vazia, esperado ao menos um par url=segredo quando MARKUPP_PUBLIC_URL existe")
	}
	return resources, nil
}

// appendUniqueResource recusa a segunda ocorrência da mesma URL, comparada como
// texto. O erro cita as posições dos pares, nunca a URL, que pode ser um segredo.
func appendUniqueResource(resources []OAuthResource, resource OAuthResource) ([]OAuthResource, error) {
	for i, existing := range resources {
		if existing.URL == resource.URL {
			return nil, fmt.Errorf("MARKUPP_OAUTH_RESOURCES com o par %d repetindo a url do par %d, esperado cada url uma vez so",
				len(resources)+1, i+1)
		}
	}
	return append(resources, resource), nil
}

// parseOAuthResource separa url e segredo no primeiro "=". Os erros citam a posição
// do par, nunca o par nem o lado esquerdo, que pode ser o segredo (par invertido ou
// segredo base64 com "=" e sem url).
func parseOAuthResource(entry string, position int) (OAuthResource, error) {
	rawURL, secret, _ := strings.Cut(entry, "=")
	rawURL, secret = strings.TrimSpace(rawURL), strings.TrimSpace(secret)
	if rawURL == "" || secret == "" || checkOAuthURL(rawURL, resourceURLRule) != nil {
		return OAuthResource{}, fmt.Errorf("MARKUPP_OAUTH_RESOURCES com o par %d fora do formato, esperado %s",
			position, resourceURLRule.expected)
	}
	if len(secret) < minOAuthResourceSecretBytes {
		return OAuthResource{}, fmt.Errorf("MARKUPP_OAUTH_RESOURCES com segredo de %d bytes no par %d, esperado ao menos %d bytes aleatorios",
			len(secret), position, minOAuthResourceSecretBytes)
	}
	return OAuthResource{URL: rawURL, Secret: secret}, nil
}
