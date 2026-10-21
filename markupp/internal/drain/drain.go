// Package drain esvazia uma fila no banco em laço: processa enquanto houver item e,
// com a fila vazia, espera e recomeça. É o modo de laço do indexador e do
// notificador no compose, onde não há KEDA (ADR-0032).
package drain

import (
	"context"
	"time"
)

// Run chama next até ele devolver false, chama wait e recomeça. Termina no primeiro
// erro de next ou de wait.
func Run(ctx context.Context, next func(context.Context) (bool, error), wait func(context.Context) error) error {
	for {
		found, err := next(ctx)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		if err := wait(ctx); err != nil {
			return err
		}
	}
}

// WaitInterval espera d, ou volta antes com o erro do contexto se ele acabar.
func WaitInterval(d time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
}
