package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

func (f *fakeAdminService) ChangeRole(ctx context.Context, actor identity.Actor, userID, role string) error {
	f.changedUser, f.changedRole = userID, role
	return f.memberErr
}

func (f *fakeAdminService) RemoveMember(ctx context.Context, actor identity.Actor, userID string) error {
	f.changedUser, f.changedRole = userID, ""
	return f.memberErr
}

func TestMembers_TrocarPapel_Retorna204(t *testing.T) {
	svc := &fakeAdminService{}

	rec := adminRequest(svc, "admin", http.MethodPatch, "/users/u2", `{"role":"viewer"}`)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "u2", svc.changedUser)
	assert.Equal(t, "viewer", svc.changedRole)
}

func TestMembers_Remover_Retorna204(t *testing.T) {
	svc := &fakeAdminService{changedRole: "sujo"}

	rec := adminRequest(svc, "owner", http.MethodDelete, "/users/u2", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "u2", svc.changedUser)
	assert.Empty(t, svc.changedRole)
}

func TestMembers_ErrosDeDominio(t *testing.T) {
	casos := map[error]int{
		auth.ErrMembershipNotFound: http.StatusNotFound,
		auth.ErrRoleAboveOwn:       http.StatusForbidden,
		auth.ErrLastOwner:          http.StatusConflict,
		auth.ErrInvalidRole:        http.StatusBadRequest,
	}
	for errDominio, status := range casos {
		rec := adminRequest(&fakeAdminService{memberErr: errDominio}, "admin", http.MethodPatch, "/users/u2", `{"role":"x"}`)

		assert.Equal(t, status, rec.Code, errDominio.Error())
	}
}
