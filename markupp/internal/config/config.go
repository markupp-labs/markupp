// Package config carrega os parâmetros de execução do servidor a partir de
// variáveis de ambiente, com valores padrão embutidos.
package config

import (
	"fmt"
	"math"
	"net/mail"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LookupFunc tem a assinatura de os.LookupEnv, para o teste injetar o ambiente.
type LookupFunc func(key string) (string, bool)

// Config são os parâmetros de execução do servidor.
type Config struct {
	Port        int
	DatabaseURL string
	MaxNoteSize int64
	// AllowedOrigins libera CORS para essas origens. Vazio desliga o CORS.
	AllowedOrigins          []string
	AuthSecret              string
	DefaultTenantID         string
	JWTExpirationMinutes    int
	RefreshExpirationDays   int
	AuthRegistrationEnabled bool
	LocalAuthEnabled        bool
	// SearchConfig é a configuração de texto do PostgreSQL usada pelo indexador.
	SearchConfig string
	// IndexInterval é a espera do index-loop entre uma rodada vazia e a próxima.
	IndexInterval time.Duration
	// ScalerPassword, quando definida, faz o migrate dar LOGIN com essa senha ao
	// papel markupp_scaler, que o KEDA usa para contar os cofres pendentes.
	ScalerPassword string
	// InviteByEmail diz à API que o convite sai por email, e então o token não volta
	// na resposta. Quem envia é o notificador, o único que recebe o SMTP.
	InviteByEmail bool
	// SMTPURL é o servidor de email do notificador, smtp:// ou smtps://.
	SMTPURL string
	// SMTPFrom é o remetente dos emails, obrigatório com SMTPURL.
	SMTPFrom string
	// PanelURL é a URL base do painel web. Definida, os emails do notificador levam o
	// link da rota do painel que recebe o token; vazia, só o token.
	PanelURL string
	// RateLimitPerMinute é quantas tentativas por minuto cada IP e cada email têm nas
	// rotas de conta (ADR-0037).
	RateLimitPerMinute int
	// TrustedProxies são as redes dos proxies cujo X-Forwarded-For o limite de
	// tentativas aceita. Vazia, o limite usa só o IP da conexão.
	TrustedProxies []netip.Prefix
}

// Default devolve a configuração usada nas variáveis não definidas.
func Default() Config {
	return Config{
		Port:                    8080,
		MaxNoteSize:             50 * 1024 * 1024,
		AuthSecret:              "",
		DefaultTenantID:         "default",
		JWTExpirationMinutes:    15,
		RefreshExpirationDays:   7,
		AuthRegistrationEnabled: false,
		LocalAuthEnabled:        true,
		SearchConfig:            "simple",
		IndexInterval:           30 * time.Second,
		RateLimitPerMinute:      5,
	}
}

// Load lê as variáveis MARKUPP_*, exigindo MARKUPP_DATABASE_URL e caindo em
// Default nas demais.
func Load(lookup LookupFunc) (Config, error) {
	cfg, err := LoadIndexer(lookup)
	if err != nil {
		return Config{}, err
	}
	readString(lookup, "MARKUPP_SCALER_PASSWORD", &cfg.ScalerPassword)
	cfg.AllowedOrigins = splitOrigins(lookup)
	port := int64(cfg.Port)
	if err := readBounded(lookup, "MARKUPP_PORT", 1, 65535, "de 1 a 65535", &port); err != nil {
		return Config{}, err
	}
	cfg.Port = int(port)
	if err := readBounded(lookup, "MARKUPP_MAX_NOTE_SIZE", 1, math.MaxInt64, "maior que zero", &cfg.MaxNoteSize); err != nil {
		return Config{}, err
	}
	if err := loadServerSettings(lookup, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// loadServerSettings lê o que só o servidor e o notificador usam: autenticação e email.
func loadServerSettings(lookup LookupFunc, cfg *Config) error {
	if err := loadAuthSettings(lookup, cfg); err != nil {
		return err
	}
	if err := loadSMTPSettings(lookup, cfg); err != nil {
		return err
	}
	if err := readRateLimit(lookup, &cfg.RateLimitPerMinute); err != nil {
		return err
	}
	if err := readTrustedProxies(lookup, &cfg.TrustedProxies); err != nil {
		return err
	}
	return readPanelURL(lookup, &cfg.PanelURL)
}

func readRateLimit(lookup LookupFunc, target *int) error {
	limit := int64(*target)
	if err := readBounded(lookup, "MARKUPP_RATE_LIMIT_PER_MINUTE", 1, 10000, "de 1 a 10000", &limit); err != nil {
		return err
	}
	*target = int(limit)
	return nil
}

// readTrustedProxies lê a lista de CIDRs separada por vírgula, ignorando itens vazios.
func readTrustedProxies(lookup LookupFunc, target *[]netip.Prefix) error {
	raw, _ := lookup("MARKUPP_TRUSTED_PROXIES")
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			return fmt.Errorf("MARKUPP_TRUSTED_PROXIES=%q: %q nao e CIDR; esperada lista de CIDRs IPv4 ou IPv6 separada por virgula, como 10.0.0.0/8,fd00::/8", raw, item)
		}
		*target = append(*target, prefix.Masked())
	}
	return nil
}

// readPanelURL aceita só URL absoluta http ou https sem query nem fragmento, porque
// o link do email acrescenta a rota e ?token= no fim dela.
func readPanelURL(lookup LookupFunc, target *string) error {
	var raw string
	readString(lookup, "MARKUPP_PANEL_URL", &raw)
	if raw == "" {
		return nil
	}
	if problem := panelURLProblem(raw); problem != "" {
		return fmt.Errorf("MARKUPP_PANEL_URL=%q: %s; esperada URL absoluta http:// ou https:// sem query nem fragmento, como https://painel.exemplo.com", raw, problem)
	}
	*target = raw
	return nil
}

// panelURLProblem diz qual regra da URL do painel raw viola, ou "" quando ela serve.
func panelURLProblem(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "nao e URL valida (" + err.Error() + ")"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "esquema " + strconv.Quote(parsed.Scheme) + " nao e http nem https"
	}
	if parsed.Host == "" {
		return "falta o host"
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return "tem query"
	}
	if parsed.Fragment != "" {
		return "tem fragmento"
	}
	return ""
}

// loadSMTPSettings lê o convite por email da API e o servidor de email do notificador
// (ADR-0021).
func loadSMTPSettings(lookup LookupFunc, cfg *Config) error {
	if err := readBool(lookup, "MARKUPP_INVITE_BY_EMAIL", &cfg.InviteByEmail); err != nil {
		return err
	}
	readString(lookup, "MARKUPP_SMTP_URL", &cfg.SMTPURL)
	readString(lookup, "MARKUPP_SMTP_FROM", &cfg.SMTPFrom)
	if cfg.SMTPURL == "" {
		return nil
	}
	if err := checkSMTPURL(cfg.SMTPURL); err != nil {
		return err
	}
	if _, err := mail.ParseAddress(cfg.SMTPFrom); err != nil {
		return fmt.Errorf("MARKUPP_SMTP_FROM=%q, esperado o remetente usuario@dominio ou Nome <usuario@dominio> quando MARKUPP_SMTP_URL existe: %w", cfg.SMTPFrom, err)
	}
	return nil
}

// checkSMTPURL recusa URL sem esquema smtp ou smtps ou sem host, sem repetir a senha.
func checkSMTPURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err == nil && (parsed.Scheme == "smtp" || parsed.Scheme == "smtps") && parsed.Hostname() != "" {
		return nil
	}
	shown := "valor ilegivel"
	if err == nil {
		shown = parsed.Redacted()
	}
	return fmt.Errorf("MARKUPP_SMTP_URL=%q, esperado smtp://usuario:senha@host:porta ou smtps://", shown)
}

// LoadIndexer lê só o que o indexador usa: o banco, a configuração de busca e o
// intervalo. O indexador não assina token, então não exige MARKUPP_AUTH_SECRET.
func LoadIndexer(lookup LookupFunc) (Config, error) {
	cfg := Default()
	url, ok := lookup("MARKUPP_DATABASE_URL")
	if !ok || url == "" {
		return Config{}, fmt.Errorf("MARKUPP_DATABASE_URL vazia, esperado postgres://usuario:senha@host:5432/banco")
	}
	cfg.DatabaseURL = url
	readString(lookup, "MARKUPP_SEARCH_CONFIG", &cfg.SearchConfig)
	if err := readInterval(lookup, "MARKUPP_INDEX_INTERVAL", &cfg.IndexInterval); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// readInterval lê key como duração positiva do Go, como 30s ou 2m.
func readInterval(lookup LookupFunc, key string, target *time.Duration) error {
	raw, ok := lookup(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("%s=%q, esperada duracao positiva como 30s ou 2m: %w", key, raw, err)
	}
	if value <= 0 {
		return fmt.Errorf("%s=%q, esperada duracao positiva como 30s ou 2m", key, raw)
	}
	*target = value
	return nil
}

// readBounded lê key como inteiro entre lowest e highest. Variável ausente ou
// vazia deixa target com o valor que já tinha.
func readBounded(lookup LookupFunc, key string, lowest, highest int64, faixa string, target *int64) error {
	raw, ok := lookup(key)
	if !ok || raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("%s=%q, esperado inteiro %s: %w", key, raw, faixa, err)
	}
	if value < lowest || value > highest {
		return fmt.Errorf("%s=%d fora da faixa, esperado inteiro %s", key, value, faixa)
	}
	*target = value
	return nil
}

func splitOrigins(lookup LookupFunc) []string {
	raw, _ := lookup("MARKUPP_ALLOWED_ORIGINS")
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

const minAuthSecretBytes = 32

func loadAuthSettings(lookup LookupFunc, cfg *Config) error {
	if err := readAuthSecret(lookup, &cfg.AuthSecret); err != nil {
		return err
	}
	readString(lookup, "MARKUPP_DEFAULT_TENANT_ID", &cfg.DefaultTenantID)
	jwtExp := int64(cfg.JWTExpirationMinutes)
	if err := readBounded(lookup, "MARKUPP_JWT_EXPIRATION_MINUTES", 1, 10080, "de 1 a 10080", &jwtExp); err != nil {
		return err
	}
	cfg.JWTExpirationMinutes = int(jwtExp)
	refExp := int64(cfg.RefreshExpirationDays)
	if err := readBounded(lookup, "MARKUPP_REFRESH_EXPIRATION_DAYS", 1, 365, "de 1 a 365", &refExp); err != nil {
		return err
	}
	cfg.RefreshExpirationDays = int(refExp)
	return loadAuthFlags(lookup, cfg)
}

func loadAuthFlags(lookup LookupFunc, cfg *Config) error {
	if err := readBool(lookup, "MARKUPP_AUTH_REGISTRATION_ENABLED", &cfg.AuthRegistrationEnabled); err != nil {
		return err
	}
	return readBool(lookup, "MARKUPP_AUTH_LOCAL_ENABLED", &cfg.LocalAuthEnabled)
}

func readAuthSecret(lookup LookupFunc, target *string) error {
	val, ok := lookup("MARKUPP_AUTH_SECRET")
	trimmed := strings.TrimSpace(val)
	if !ok || trimmed == "" {
		return fmt.Errorf("MARKUPP_AUTH_SECRET vazia, esperado ao menos %d bytes aleatorios (ex.: openssl rand -hex 32)", minAuthSecretBytes)
	}
	if len(trimmed) < minAuthSecretBytes {
		return fmt.Errorf("MARKUPP_AUTH_SECRET com %d bytes, esperado ao menos %d bytes aleatorios", len(trimmed), minAuthSecretBytes)
	}
	*target = trimmed
	return nil
}

func readString(lookup LookupFunc, key string, target *string) {
	if val, ok := lookup(key); ok && strings.TrimSpace(val) != "" {
		*target = strings.TrimSpace(val)
	}
}

func readBool(lookup LookupFunc, key string, target *bool) error {
	val, ok := lookup(key)
	if !ok || strings.TrimSpace(val) == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return fmt.Errorf("%s=%q invalido, esperado booleano (true/false/1/0): %w", key, val, err)
	}
	*target = parsed
	return nil
}
