package vaults_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaults"
)

// fakeVaultsRepository guarda o que o serviço manda gravar e devolve o erro configurado.
type fakeVaultsRepository struct {
	created  vaults.Vault
	renamed  [2]string
	deleted  string
	added    [2]string
	addedAt  time.Time
	removed  [2]string
	event    auth.AuditEventRecord
	writeErr error
}

func (f *fakeVaultsRepository) List(ctx context.Context) ([]vaults.Vault, error) {
	return []vaults.Vault{{ID: "v1", Name: "Pessoal"}}, nil
}

func (f *fakeVaultsRepository) CreateTx(ctx context.Context, v vaults.Vault, event auth.AuditEventRecord) error {
	f.created, f.event = v, event
	return f.writeErr
}

func (f *fakeVaultsRepository) RenameTx(ctx context.Context, id, name string, event auth.AuditEventRecord) (vaults.Vault, error) {
	f.renamed, f.event = [2]string{id, name}, event
	return vaults.Vault{ID: id, Name: name}, f.writeErr
}

func (f *fakeVaultsRepository) DeleteTx(ctx context.Context, id string, event auth.AuditEventRecord) error {
	f.deleted, f.event = id, event
	return f.writeErr
}

func (f *fakeVaultsRepository) ListMembers(ctx context.Context, vaultID string) ([]vaults.Member, error) {
	return []vaults.Member{{UserID: "u1"}}, nil
}

func (f *fakeVaultsRepository) AddMemberTx(ctx context.Context, vaultID, userID string, at time.Time, event auth.AuditEventRecord) (vaults.Member, error) {
	f.added, f.addedAt, f.event = [2]string{vaultID, userID}, at, event
	return vaults.Member{UserID: userID}, f.writeErr
}

func (f *fakeVaultsRepository) RemoveMemberTx(ctx context.Context, vaultID, userID string, event auth.AuditEventRecord) error {
	f.removed, f.event = [2]string{vaultID, userID}, event
	return f.writeErr
}

var (
	momento    = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	membroAtor = identity.Actor{UserID: "u1", TenantID: "t1", Role: identity.RoleMember}
	viewerAtor = identity.Actor{UserID: "u9", TenantID: "t1", Role: identity.RoleViewer}
)

func novoServico(repo *fakeVaultsRepository) *vaults.Service {
	ids := []string{"id-1", "id-2"}
	newID := func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}
	return vaults.NewService(repo, func() time.Time { return momento }, newID)
}

func eventoEsperado(id, action, target string) auth.AuditEventRecord {
	return auth.AuditEventRecord{
		ID: id, TenantID: "t1", ActorID: "u1", Action: action, Target: target, Result: "success", OccurredAt: momento,
	}
}

func TestCreate_NomeComEspacos_GravaAparadoComOAtorEAudita(t *testing.T) {
	repo := &fakeVaultsRepository{}

	v, err := novoServico(repo).Create(context.Background(), membroAtor, "  Projetos  ")

	require.NoError(t, err)
	assert.Equal(t, vaults.Vault{ID: "id-1", Name: "Projetos", CreatedAt: momento}, v)
	assert.Equal(t, v, repo.created)
	assert.Equal(t, eventoEsperado("id-2", "vaults.create", "id-1"), repo.event)
}

func TestCreate_NomeVazioOuLongo_RecusaComOValor(t *testing.T) {
	for _, nome := range []string{"   ", strings.Repeat("a", 101)} {
		_, err := novoServico(&fakeVaultsRepository{}).Create(context.Background(), membroAtor, nome)

		require.ErrorIs(t, err, vaults.ErrInvalidName)
		assert.Contains(t, err.Error(), "de 1 a 100 caracteres")
	}
}

func TestCreate_NomeCom100Caracteres_Aceita(t *testing.T) {
	_, err := novoServico(&fakeVaultsRepository{}).Create(context.Background(), membroAtor, strings.Repeat("a", 100))

	require.NoError(t, err)
}

func TestRename_GravaONomeAparadoEAudita(t *testing.T) {
	repo := &fakeVaultsRepository{}

	v, err := novoServico(repo).Rename(context.Background(), membroAtor, "v1", " Novo ")

	require.NoError(t, err)
	assert.Equal(t, "Novo", v.Name)
	assert.Equal(t, [2]string{"v1", "Novo"}, repo.renamed)
	assert.Equal(t, eventoEsperado("id-1", "vaults.rename", "v1"), repo.event)
}

func TestRename_NomeInvalido_NaoGrava(t *testing.T) {
	repo := &fakeVaultsRepository{}

	_, err := novoServico(repo).Rename(context.Background(), membroAtor, "v1", "")

	require.ErrorIs(t, err, vaults.ErrInvalidName)
	assert.Empty(t, repo.renamed[0])
}

func TestDelete_ApagaEAudita(t *testing.T) {
	repo := &fakeVaultsRepository{}

	err := novoServico(repo).Delete(context.Background(), membroAtor, "v1")

	require.NoError(t, err)
	assert.Equal(t, "v1", repo.deleted)
	assert.Equal(t, eventoEsperado("id-1", "vaults.delete", "v1"), repo.event)
}

func TestAddMember_GravaComOMomentoEAuditaNaConta(t *testing.T) {
	repo := &fakeVaultsRepository{}

	m, err := novoServico(repo).AddMember(context.Background(), membroAtor, "v1", "u2")

	require.NoError(t, err)
	assert.Equal(t, "u2", m.UserID)
	assert.Equal(t, [2]string{"v1", "u2"}, repo.added)
	assert.Equal(t, momento, repo.addedAt)
	assert.Equal(t, eventoEsperado("id-1", "vaults.members.add", "u2"), repo.event)
}

func TestRemoveMember_TiraEAuditaNaConta(t *testing.T) {
	repo := &fakeVaultsRepository{}

	err := novoServico(repo).RemoveMember(context.Background(), membroAtor, "v1", "u2")

	require.NoError(t, err)
	assert.Equal(t, [2]string{"v1", "u2"}, repo.removed)
	assert.Equal(t, eventoEsperado("id-1", "vaults.members.remove", "u2"), repo.event)
}

func TestEscritas_Viewer_RecusaComErrReadOnlySemGravar(t *testing.T) {
	repo := &fakeVaultsRepository{}
	svc := novoServico(repo)
	ctx := context.Background()

	_, errCreate := svc.Create(ctx, viewerAtor, "Novo")
	_, errRename := svc.Rename(ctx, viewerAtor, "v1", "Novo")
	errDelete := svc.Delete(ctx, viewerAtor, "v1")
	_, errAdd := svc.AddMember(ctx, viewerAtor, "v1", "u2")
	errRemove := svc.RemoveMember(ctx, viewerAtor, "v1", "u2")

	for _, err := range []error{errCreate, errRename, errDelete, errAdd, errRemove} {
		require.ErrorIs(t, err, vaults.ErrReadOnly)
		assert.Contains(t, err.Error(), `"viewer"`)
	}
	assert.Equal(t, &fakeVaultsRepository{}, repo, "o viewer nao chega ao repositorio")
}

func TestRemoveMember_ViewerSaindoSozinho_Sai(t *testing.T) {
	repo := &fakeVaultsRepository{}

	err := novoServico(repo).RemoveMember(context.Background(), viewerAtor, "v1", "u9")

	require.NoError(t, err)
	assert.Equal(t, [2]string{"v1", "u9"}, repo.removed)
}

func TestLeituras_RepassamORepositorio(t *testing.T) {
	svc := novoServico(&fakeVaultsRepository{})

	lista, errList := svc.List(context.Background())
	membros, errMembers := svc.ListMembers(context.Background(), "v1")

	require.NoError(t, errList)
	require.NoError(t, errMembers)
	assert.Equal(t, "v1", lista[0].ID)
	assert.Equal(t, "u1", membros[0].UserID)
}
