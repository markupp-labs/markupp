package notify

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPortOrDefault_SemPorta_UsaASubmissaoDoEsquema(t *testing.T) {
	casos := map[string]string{"smtp://h": "587", "smtps://h": "465", "smtp://h:2525": "2525"}
	for raw, porta := range casos {
		u, _ := url.Parse(raw)

		assert.Equal(t, porta, portOrDefault(u), raw)
	}
}
