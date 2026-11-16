package linkrefusals_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/linkrefusals"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// FakeRefusalStore guarda as chamadas que chegam à persistência.
type FakeRefusalStore struct {
	created [][2]string
	deleted []string
}

func (f *FakeRefusalStore) Create(_ context.Context, _, noteA, noteB string) (linkrefusals.Refusal, error) {
	f.created = append(f.created, [2]string{noteA, noteB})
	return linkrefusals.Refusal{ID: "r1"}, nil
}

func (f *FakeRefusalStore) List(context.Context, string, notes.Page) ([]linkrefusals.Refusal, error) {
	return []linkrefusals.Refusal{{ID: "r1"}}, nil
}

func (f *FakeRefusalStore) Delete(_ context.Context, _, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

var (
	editor = identity.Actor{UserID: "u1", TenantID: "t1", Role: "member"}
	viewer = identity.Actor{UserID: "u2", TenantID: "t1", Role: identity.RoleViewer}
)

func TestRefuse_EditorGrava(t *testing.T) {
	store := &FakeRefusalStore{}

	recusa, err := linkrefusals.NewService(store).Refuse(context.Background(), editor, "v1", "a", "b")

	require.NoError(t, err)
	assert.Equal(t, "r1", recusa.ID)
	assert.Equal(t, [][2]string{{"a", "b"}}, store.created)
}

func TestRefuse_ViewerSoLe(t *testing.T) {
	store := &FakeRefusalStore{}
	svc := linkrefusals.NewService(store)

	_, err := svc.Refuse(context.Background(), viewer, "v1", "a", "b")
	require.ErrorIs(t, err, linkrefusals.ErrReadOnly)
	require.ErrorIs(t, svc.Undo(context.Background(), viewer, "v1", "r1"), linkrefusals.ErrReadOnly)

	lista, err := svc.List(context.Background(), "v1", notes.Page{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, lista, 1)
	assert.Empty(t, store.created)
	assert.Empty(t, store.deleted)
}

func TestRefuse_MesmaNotaDosDoisLados(t *testing.T) {
	store := &FakeRefusalStore{}

	_, err := linkrefusals.NewService(store).Refuse(context.Background(), editor, "v1", "a", "a")

	require.ErrorIs(t, err, linkrefusals.ErrSameNote)
	assert.ErrorContains(t, err, `"a"`)
	assert.Empty(t, store.created)
}

func TestUndo_EditorApaga(t *testing.T) {
	store := &FakeRefusalStore{}

	require.NoError(t, linkrefusals.NewService(store).Undo(context.Background(), editor, "v1", "r1"))

	assert.Equal(t, []string{"r1"}, store.deleted)
}
