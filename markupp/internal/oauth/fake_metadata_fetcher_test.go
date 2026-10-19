package oauth_test

import (
	"context"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// fakeClientMetadataFetcher devolve o documento ou o erro configurado e guarda cada
// client_id buscado.
type fakeClientMetadataFetcher struct {
	metadata oauth.ClientMetadata
	err      error
	calls    []string
}

func (f *fakeClientMetadataFetcher) FetchClientMetadata(_ context.Context, clientID string) (oauth.ClientMetadata, error) {
	f.calls = append(f.calls, clientID)
	if f.err != nil {
		return oauth.ClientMetadata{}, f.err
	}
	return f.metadata, nil
}
