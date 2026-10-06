// Package config carrega os parâmetros de execução do servidor a partir de
// variáveis de ambiente, com valores padrão embutidos.
package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// LookupFunc tem a assinatura de os.LookupEnv, para o teste injetar o ambiente.
type LookupFunc func(key string) (string, bool)

// Config são os parâmetros de execução do servidor.
type Config struct {
	Port        int
	DatabaseURL string
	MaxNoteSize int64
	// AllowedOrigins libera CORS para essas origens. Vazio desliga o CORS.
	AllowedOrigins []string
}

// Default devolve a configuração usada nas variáveis não definidas.
func Default() Config {
	return Config{
		Port:        8080,
		MaxNoteSize: 50 * 1024 * 1024,
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
	err := readBounded(lookup, "MARKUPP_MAX_NOTE_SIZE", 1, math.MaxInt64, "maior que zero", &cfg.MaxNoteSize)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
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
