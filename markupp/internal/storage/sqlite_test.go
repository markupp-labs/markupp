package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func TestOpenDB_BancoEmMemoria_RetornaConexaoRespondendoAoPing(t *testing.T) {
	db, err := storage.OpenDB(":memory:")

	require.NoError(t, err)
	require.NotNil(t, db)
	t.Cleanup(func() { _ = db.Close() })
	assert.NoError(t, db.Ping())
}

func TestOpenDB_DiretorioInexistente_RetornaErroENilDB(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "sem-esse-diretorio", "markupp.db")

	db, err := storage.OpenDB(caminho)

	require.Error(t, err, "esperado erro de conexao para o caminho %q", caminho)
	assert.Nil(t, db)
}

func TestMigrate_BancoEmMemoria_CriaTabelaNotes(t *testing.T) {
	db, err := storage.OpenDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, storage.Migrate(db))

	var tabela string
	consulta := "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'notes'"
	require.NoError(t, db.QueryRow(consulta).Scan(&tabela))
	assert.Equal(t, "notes", tabela)
}

func TestMigrate_ConexaoFechada_RetornaErro(t *testing.T) {
	db, err := storage.OpenDB(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	err = storage.Migrate(db)

	assert.Error(t, err, "esperado erro ao migrar sobre conexao fechada")
}
