package notes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type repoComInstanteFixo struct {
	Repository
	armazenada     Note
	updatedAtVisto time.Time
}

func (r *repoComInstanteFixo) GetNoteByID(ctx context.Context, id string) (Note, error) {
	return r.armazenada, nil
}

func (r *repoComInstanteFixo) Update(ctx context.Context, id, path, content string, updatedAt, lastModifiedAt time.Time, force bool) (Note, error) {
	r.updatedAtVisto = updatedAt
	return r.armazenada, nil
}

func TestStrictlyAfter_AgoraMaior_RetornaAgora(t *testing.T) {
	anterior := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	agora := anterior.Add(5 * time.Millisecond)

	assert.Equal(t, agora, strictlyAfter(anterior, agora))
}

func TestStrictlyAfter_MesmoInstante_AvancaUmMilissegundo(t *testing.T) {
	anterior := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	assert.Equal(t, anterior.Add(time.Millisecond), strictlyAfter(anterior, anterior))
}

func TestStrictlyAfter_RelogioAtrasado_AvancaUmMilissegundo(t *testing.T) {
	anterior := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	agora := anterior.Add(-3 * time.Millisecond)

	assert.Equal(t, anterior.Add(time.Millisecond), strictlyAfter(anterior, agora))
}

func TestUpdate_RelogioParadoNoInstanteArmazenado_AvancaUpdatedAt(t *testing.T) {
	instante := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	repo := &repoComInstanteFixo{armazenada: Note{ID: "n1", Path: "a.md", UpdatedAt: instante}}
	svc := NewService(repo, 100)
	svc.clock = func() time.Time { return instante }

	_, err := svc.Update(context.Background(), "n1", "a.md", "v2", instante, false)

	require.NoError(t, err)
	assert.True(t, repo.updatedAtVisto.After(instante),
		"updated_at gravado (%s) precisa ser estritamente maior que o anterior (%s)",
		repo.updatedAtVisto, instante)
}
