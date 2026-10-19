package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// maxCIMDClientIDLen limita o client_id tratado como URL de documento CIMD, que chega de
// qualquer pedido sem login.
const maxCIMDClientIDLen = 2048

// metadataDocumentShape descreve o documento esperado quando a busca do CIMD falha.
const metadataDocumentShape = "um Client ID Metadata Document acessivel e valido (draft-ietf-oauth-client-id-metadata-document)"

// resolveClient acha o cliente do client_id: por CIMD quando ele é URL https e há
// fetcher, senão pelo cadastro do repositório.
func (s *Service) resolveClient(ctx context.Context, clientID string) (Client, error) {
	if s.metadata == nil || !isHTTPSURL(clientID) {
		return s.registeredClient(ctx, clientID)
	}
	if len(clientID) > maxCIMDClientIDLen {
		return Client{}, &ProtocolError{Code: errInvalidRequest, Description: fmt.Sprintf("client_id com %d caracteres; esperado no maximo %d", len(clientID), maxCIMDClientIDLen)}
	}
	return s.cimdClient(ctx, clientID)
}

// isHTTPSURL diz se o client_id tem a forma de URL https com host.
func isHTTPSURL(clientID string) bool {
	parsed, err := url.Parse(clientID)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// registeredClient busca o cliente cadastrado, por CIMD ou DCR, sem buscar documento.
func (s *Service) registeredClient(ctx context.Context, clientID string) (Client, error) {
	client, err := s.repo.GetClient(ctx, clientID, time.Time{})
	if errors.Is(err, ErrClientNotFound) {
		return Client{}, &ProtocolError{Code: errInvalidRequest, Description: "client_id " + ClientValue(clientID) + " nao registrado; esperado " + registeredClientShape}
	}
	if err != nil {
		return Client{}, fmt.Errorf("oauth: buscando o cliente %s: %w", ClientValue(clientID), err)
	}
	return client, nil
}

// cimdClient usa o documento em cache ainda válido ou busca e grava um novo. Documento
// vencido com a busca falhando recusa o pedido, sem cair na cópia vencida.
func (s *Service) cimdClient(ctx context.Context, clientID string) (Client, error) {
	now := s.clock()
	client, err := s.repo.GetFreshCIMDClient(ctx, clientID, now)
	if err == nil {
		return client, nil
	}
	if !errors.Is(err, ErrClientNotFound) {
		return Client{}, fmt.Errorf("oauth: lendo o cache do cliente cimd %s: %w", ClientValue(clientID), err)
	}
	return s.fetchCIMDClient(ctx, clientID, now)
}

// fetchCIMDClient busca o documento e grava o cliente até o fim do TTL dele. A falha da
// busca vai só para o log, como causa interna, para não servir de oráculo da rede
// interna; o documento recusado pelas guardas daqui não entra no cache negativo.
func (s *Service) fetchCIMDClient(ctx context.Context, clientID string, now time.Time) (Client, error) {
	if until, failed := s.failures.failedUntil(clientID, now); failed {
		return Client{}, unusableMetadataError(clientID, fmt.Errorf("oauth: busca do documento cimd de %s falhou ha pouco; esperada nova tentativa a partir de %s",
			ClientValue(clientID), until.Format(time.RFC3339)))
	}
	metadata, err := s.fetchMetadata(ctx, clientID, now)
	if err != nil {
		return Client{}, unusableMetadataError(clientID, err)
	}
	if err := checkFetchedMetadata(clientID, metadata); err != nil {
		return Client{}, unusableMetadataError(clientID, err)
	}
	return s.saveCIMDClient(ctx, clientID, metadata, now)
}

// fetchMetadata busca o documento pelo cimdFetchGate e lembra por CIMDFailureTTL a falha
// que passou pela rede: nem o client_id recusado antes da busca nem a falta de vaga
// entram no cache negativo. A busca não herda o cancelamento do pedido, só o timeout do próprio fetcher:
// senão quem derruba a conexão faria a busca de um cliente legítimo falhar e cair no
// cache negativo.
func (s *Service) fetchMetadata(ctx context.Context, clientID string, now time.Time) (ClientMetadata, error) {
	metadata, err := s.fetches.fetch(clientID, func() (ClientMetadata, error) {
		return s.metadata.FetchClientMetadata(context.WithoutCancel(ctx), clientID)
	})
	if err != nil && !errors.Is(err, ErrInvalidMetadataClientID) && !errors.Is(err, errCIMDFetchBusy) {
		s.failures.remember(clientID, now.Add(s.cfg.CIMDFailureTTL), now)
	}
	return metadata, err
}

// saveCIMDClient grava o cliente até o fim do TTL do documento. O conflito com cliente de
// outra origem vira a mesma recusa genérica.
func (s *Service) saveCIMDClient(ctx context.Context, clientID string, metadata ClientMetadata, now time.Time) (Client, error) {
	client := Client{ID: clientID, Name: metadata.Name, RedirectURIs: metadata.RedirectURIs}
	err := s.repo.SaveCIMDClient(ctx, client, now.Add(metadata.TTL), now)
	if errors.Is(err, ErrClientSourceConflict) {
		return Client{}, unusableMetadataError(clientID, err)
	}
	if err != nil {
		return Client{}, fmt.Errorf("oauth: gravando o cliente cimd %s: %w", ClientValue(clientID), err)
	}
	return client, nil
}

// unusableMetadataError é a recusa genérica do documento CIMD, igual para toda falha;
// cause fica só para o log.
func unusableMetadataError(clientID string, cause error) error {
	recusa := &ProtocolError{Code: errInvalidRequest, Description: "documento de metadados do client_id " + ClientValue(clientID) + " inacessivel ou invalido; esperado " + metadataDocumentShape}
	return recusa.WithInternalCause(cause)
}

// checkFetchedMetadata não confia no fetcher: o documento tem de ser do client_id pedido,
// ter redirect_uris e um TTL de cache positivo.
func checkFetchedMetadata(clientID string, metadata ClientMetadata) error {
	switch {
	case metadata.ClientID != clientID:
		return fmt.Errorf("oauth: documento cimd com client_id %s; esperado %s", ClientValue(metadata.ClientID), ClientValue(clientID))
	case len(metadata.RedirectURIs) == 0:
		return fmt.Errorf("oauth: documento cimd de %s sem redirect_uris; esperada ao menos uma", ClientValue(clientID))
	case metadata.TTL <= 0:
		return fmt.Errorf("oauth: documento cimd de %s com TTL %s; esperado maior que zero", ClientValue(clientID), metadata.TTL)
	}
	return nil
}
