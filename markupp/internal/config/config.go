// Package config carrega os parâmetros de execução do servidor a partir de
// um arquivo JSON, com valores padrão embutidos.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
)

const (
	envVarName  = "MARKUPP_CONFIG_PATH"
	defaultPath = "./config.json"
)

// Config são os parâmetros de execução do servidor.
type Config struct {
	Port        int    `json:"port"`
	DBPath      string `json:"db_path"`
	MaxNoteSize int64  `json:"max_note_size"`
}

// Default devolve a configuração usada quando não há arquivo de config.
func Default() Config {
	return Config{
		Port:        8080,
		DBPath:      "./markupp.db",
		MaxNoteSize: 50 * 1024 * 1024,
	}
}

// Load lê o JSON apontado por MARKUPP_CONFIG_PATH, caindo em Default quando o
// arquivo não existe.
func Load() (Config, error) {
	path := os.Getenv(envVarName)
	if path == "" {
		path = defaultPath
	}

	cfg := Default()
	data, err := os.ReadFile(path) //nolint:gosec // caminho vem de configuracao do operador via MARKUPP_CONFIG_PATH, nao de entrada de usuario
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
