package auth

import (
	"errors"
	"fmt"
	"time"
)

// InviteWindow é a janela em que os convites emitidos contam para o limite (#50).
const InviteWindow = 24 * time.Hour

// Erros do limite de convites.
var (
	ErrInviteLimit      = errors.New("invite limit reached")
	ErrEmailNotVerified = errors.New("email of the account is not verified")
)

// InviteLimits é quantos convites cabem na janela, por conta somando todos os tenants
// dela e por tenant somando todas as contas.
type InviteLimits struct {
	PerAccount int
	PerTenant  int
}

// InviteQuota é o que a transação do convite vê: se quem convida provou o email e
// quantos convites já saíram na janela. Convite revogado continua contando.
type InviteQuota struct {
	InviterVerified bool
	ByInviter       int
	ByTenant        int
}

// CheckInviteQuota barra o convite que estoura um limite e, quando o convite sai por
// email, o de conta que ainda não provou o email: os dois juntos impedem que o
// remetente da instância vire relay de spam e phishing.
func CheckInviteQuota(q InviteQuota, l InviteLimits, byEmail bool) error {
	if byEmail && !q.InviterVerified {
		return fmt.Errorf("%w: confirm the email of the account before inviting", ErrEmailNotVerified)
	}
	if q.ByInviter >= l.PerAccount {
		return fmt.Errorf("%w: %d invites by this account in %s, limit %d", ErrInviteLimit, q.ByInviter, windowLabel(), l.PerAccount)
	}
	if q.ByTenant >= l.PerTenant {
		return fmt.Errorf("%w: %d invites in this tenant in %s, limit %d", ErrInviteLimit, q.ByTenant, windowLabel(), l.PerTenant)
	}
	return nil
}

func windowLabel() string {
	return fmt.Sprintf("%.0fh", InviteWindow.Hours())
}
