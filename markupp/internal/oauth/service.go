package oauth

import (
	"net/url"
	"strings"
	"time"
)

// Códigos de erro do protocolo usados pelo Service (RFC 6749 e RFC 8707).
const (
	errInvalidRequest          = "invalid_request"
	errUnsupportedResponseType = "unsupported_response_type"
	errInvalidScope            = "invalid_scope"
	errInvalidTarget           = "invalid_target"
	errInvalidGrant            = "invalid_grant"
	errAccessDenied            = "access_denied"
)

// maxDescribedValueLen limita o trecho do valor recebido citado num error_description.
const maxDescribedValueLen = 80

// ServiceConfig é a configuração do authorization server. Resources já vem na forma
// canônica de CanonicalURL.
type ServiceConfig struct {
	Issuer     string
	PanelURL   string
	Resources  []string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	RequestTTL time.Duration
	CodeTTL    time.Duration
}

// Service conduz o fluxo authorization code com PKCE: cria o pedido, mostra e decide o
// consentimento e troca o código por tokens.
type Service struct {
	repo     Repository
	cfg      ServiceConfig
	clock    func() time.Time
	newID    func() string
	newToken func() string
}

// NewService monta o Service. newToken gera os códigos e tokens opacos, que só chegam ao
// repositório pelo hash.
func NewService(repo Repository, cfg ServiceConfig, clock func() time.Time, newID func() string, newToken func() string) *Service {
	return &Service{repo: repo, cfg: cfg, clock: clock, newID: newID, newToken: newToken}
}

// canonicalize devolve a forma canônica de uma URL recebida; ok falso se ela não parseia.
func canonicalize(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	return CanonicalURL(parsed), true
}

// ClientValue prepara um valor vindo do cliente para citar no error_description, que
// a RFC 6749 restringe a ASCII visível sem aspas duplas nem barra invertida, e nos erros
// embrulhados que vão para o log, com o mesmo limite de tamanho.
func ClientValue(value string) string {
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return '?'
		}
		return r
	}, value)
	if len(clean) > maxDescribedValueLen {
		clean = clean[:maxDescribedValueLen] + "..."
	}
	return "'" + clean + "'"
}
