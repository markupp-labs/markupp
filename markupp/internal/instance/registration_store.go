package instance

import "context"

// RegistrationStore persiste a escolha do administrador da instância por método de
// cadastro (ADR-0059). Método sem escolha guardada não aparece no mapa.
type RegistrationStore interface {
	LoadRegistrationChoices(ctx context.Context) (map[string]bool, error)
	SaveRegistrationChoice(ctx context.Context, method string, open bool) error
}
