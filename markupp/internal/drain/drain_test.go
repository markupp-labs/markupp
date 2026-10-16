package drain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/drain"
)

func TestWaitInterval_ContextoCancelado_VoltaNaHoraComOErro(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inicio := time.Now()
	err := drain.WaitInterval(time.Hour)(ctx)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(inicio), time.Second)
}

func TestWaitInterval_IntervaloPassa_VoltaSemErro(t *testing.T) {
	assert.NoError(t, drain.WaitInterval(time.Millisecond)(context.Background()))
}

// filaRoteirizada devolve, chamada a chamada, se havia item, e conta as chamadas.
type filaRoteirizada struct {
	itens []bool
	calls int
}

func (f *filaRoteirizada) next(context.Context) (bool, error) {
	f.calls++
	if len(f.itens) == 0 {
		return false, nil
	}
	found := f.itens[0]
	f.itens = f.itens[1:]
	return found, nil
}

func TestRun_EsvaziaAFilaAntesDeEsperar(t *testing.T) {
	fila := &filaRoteirizada{itens: []bool{true, true, false, false}}
	esperas := 0

	err := drain.Run(context.Background(), fila.next, func(context.Context) error {
		esperas++
		if esperas == 2 {
			return context.Canceled
		}
		return nil
	})

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 4, fila.calls)
	assert.Equal(t, 2, esperas)
}

func TestRun_ErroDoItem_DevolveSemEsperar(t *testing.T) {
	falha := errors.New("banco fora")
	esperou := false

	err := drain.Run(context.Background(), func(context.Context) (bool, error) { return false, falha },
		func(context.Context) error { esperou = true; return nil })

	assert.ErrorIs(t, err, falha)
	assert.False(t, esperou)
}
