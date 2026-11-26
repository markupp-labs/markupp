package auth

import "context"

// Origin é de onde veio o pedido: o IP de quem usa e o user agent do cliente (ADR-0061).
type Origin struct {
	IP        string
	UserAgent string
}

type originKey struct{}

// WithOrigin guarda a origem do pedido no contexto, para a sessão e a auditoria.
func WithOrigin(ctx context.Context, origin Origin) context.Context {
	return context.WithValue(ctx, originKey{}, origin)
}

// OriginFrom devolve a origem guardada no contexto, ou vazia quando não há.
func OriginFrom(ctx context.Context) Origin {
	origin, _ := ctx.Value(originKey{}).(Origin)
	return origin
}
