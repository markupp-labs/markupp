// Package config carrega os parâmetros de execução do servidor a partir de
// variáveis de ambiente, com valores padrão embutidos.
package config

import (
	"fmt"
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

	if err := readInt(lookup, "MARKUPP_PORT", &cfg.Port); err != nil {
		return Config{}, err
	}
	var size int
	if err := readInt(lookup, "MARKUPP_MAX_NOTE_SIZE", &size); err != nil {
		return Config{}, err
	}
	if size > 0 {
		cfg.MaxNoteSize = int64(size)
	}
	return cfg, nil
}

func readInt(lookup LookupFunc, key string, target *int) error {
	raw, ok := lookup(key)
	if !ok || raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("%s=%q, esperado inteiro: %w", key, raw, err)
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
