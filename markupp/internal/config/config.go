// Package config carrega os parâmetros de execução do servidor a partir de
// variáveis de ambiente, com valores padrão embutidos.
package config

import (
	"fmt"
	"math"
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
	}
}

// Load lê as variáveis MARKUPP_*, exigindo MARKUPP_DATABASE_URL e caindo em
// Default nas demais.
func Load(lookup LookupFunc) (Config, error) {
	cfg := Default()
	url, ok := lookup("MARKUPP_DATABASE_URL")
	if !ok || url == "" {
		return Config{}, fmt.Errorf("MARKUPP_DATABASE_URL vazia, esperado postgres://usuario:senha@host:5432/banco")
	}
	cfg.DatabaseURL = url
	cfg.AllowedOrigins = splitOrigins(lookup)
	port := int64(cfg.Port)
	if err := readBounded(lookup, "MARKUPP_PORT", 1, 65535, "de 1 a 65535", &port); err != nil {
		return Config{}, err
	}
	cfg.Port = int(port)
	if err := readBounded(lookup, "MARKUPP_MAX_NOTE_SIZE", 1, math.MaxInt64, "maior que zero", &cfg.MaxNoteSize); err != nil {
		return Config{}, err
	}
	if err := loadAuthSettings(lookup, &cfg); err != nil {
		return Config{}, err
	}
	if err := loadIndexerSettings(lookup, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadIndexerSettings(lookup LookupFunc, cfg *Config) error {
	readString(lookup, "MARKUPP_SEARCH_CONFIG", &cfg.SearchConfig)
	return readInterval(lookup, "MARKUPP_INDEX_INTERVAL", &cfg.IndexInterval)
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
