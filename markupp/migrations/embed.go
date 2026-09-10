// Package migrations embute os arquivos .sql de migração para que o binário
// não dependa deles em disco.
package migrations

import "embed"

// FS expõe as migrações embutidas para o goose aplicá-las.
//
//go:embed *.sql
var FS embed.FS
