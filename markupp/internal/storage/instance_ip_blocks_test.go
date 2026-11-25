package storage_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func bloqueioDe(id, cidr string) instance.IPBlock {
	return instance.IPBlock{ID: id, Prefix: netip.MustParsePrefix(cidr), Note: "nota " + id,
		CreatedAt: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)}
}

func eventoDoBloqueio(action, id string) auth.AuditEventRecord {
	return auth.AuditEventRecord{ID: "ev-" + action + "-" + id, ActorID: "u-admin", Action: action, Target: id,
		Result: "success", OccurredAt: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)}
}

func TestInstanceIPBlocks_AdminCria_TodosLeemNaOrdem(t *testing.T) {
	repo := storage.NewInstanceIPBlocks(bancoDaInstancia(t))

	require.NoError(t, repo.CreateIPBlock(comAtor(), bloqueioDe("b1", "198.51.100.0/24"), eventoDoBloqueio("ip_blocks.create", "b1")))
	require.NoError(t, repo.CreateIPBlock(comAtor(), bloqueioDe("b2", "2001:db8::/32"), eventoDoBloqueio("ip_blocks.create", "b2")))
	blocks, err := repo.ListIPBlocks(context.Background())

	require.NoError(t, err)
	require.Len(t, blocks, 2)
	assert.Equal(t, "b1", blocks[0].ID)
	assert.Equal(t, netip.MustParsePrefix("198.51.100.0/24"), blocks[0].Prefix)
	assert.Equal(t, "nota b1", blocks[0].Note)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), blocks[1].Prefix)
}

func TestInstanceIPBlocks_FaixaRepetida_Recusa(t *testing.T) {
	repo := storage.NewInstanceIPBlocks(bancoDaInstancia(t))
	require.NoError(t, repo.CreateIPBlock(comAtor(), bloqueioDe("b1", "198.51.100.0/24"), eventoDoBloqueio("ip_blocks.create", "b1")))

	err := repo.CreateIPBlock(comAtor(), bloqueioDe("b2", "198.51.100.0/24"), eventoDoBloqueio("ip_blocks.create", "b2"))

	require.ErrorIs(t, err, instance.ErrIPBlockExists)
}

func TestInstanceIPBlocks_Apagar_ESegundaVezNaoAcha(t *testing.T) {
	repo := storage.NewInstanceIPBlocks(bancoDaInstancia(t))
	require.NoError(t, repo.CreateIPBlock(comAtor(), bloqueioDe("b1", "198.51.100.0/24"), eventoDoBloqueio("ip_blocks.create", "b1")))

	require.NoError(t, repo.DeleteIPBlock(comAtor(), "b1", eventoDoBloqueio("ip_blocks.delete", "b1")))
	err := repo.DeleteIPBlock(comAtor(), "b1", eventoDoBloqueio("ip_blocks.delete", "b1-2"))

	require.ErrorIs(t, err, instance.ErrIPBlockNotFound)
	blocks, err := repo.ListIPBlocks(context.Background())
	require.NoError(t, err)
	assert.Empty(t, blocks)
}

func TestInstanceIPBlocks_CriarSemAtor_Recusa(t *testing.T) {
	repo := storage.NewInstanceIPBlocks(bancoDaInstancia(t))

	err := repo.CreateIPBlock(context.Background(), bloqueioDe("b1", "198.51.100.0/24"), eventoDoBloqueio("ip_blocks.create", "b1"))

	require.ErrorIs(t, err, storage.ErrNoActor)
}
