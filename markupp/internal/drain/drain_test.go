package drain_test

import (
	"context"
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
