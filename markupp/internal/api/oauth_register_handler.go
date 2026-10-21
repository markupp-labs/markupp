package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"strconv"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// maxRegisterBodyBytes limita o corpo JSON do registro dinâmico, que chega sem login.
const maxRegisterBodyBytes = 16 << 10

const oauthErrInvalidClientMetadata = "invalid_client_metadata"

// register atende o registro dinâmico de clientes públicos (RFC 7591), anônimo e com o
// CORS aberto do subroteador, sem nenhuma resposta guardada em cache.
func (h *oauthHandler) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	req, protoErr := readRegistrationBody(w, r)
	if protoErr != nil {
		writeOAuthError(w, protoErr)
		return
	}
	registered, err := h.svc.RegisterClient(r.Context(), req)
	if err != nil {
		writeRegistrationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, registered)
}

// readRegistrationBody lê um único objeto JSON de até maxRegisterBodyBytes. Campos
// desconhecidos são ignorados, como pede a RFC 7591.
func readRegistrationBody(w http.ResponseWriter, r *http.Request) (oauth.RegistrationRequest, *oauth.ProtocolError) {
	contentType := r.Header.Get("Content-Type")
	if mediaType, _, _ := mime.ParseMediaType(contentType); mediaType != "application/json" {
		return oauth.RegistrationRequest{}, invalidClientMetadata(fmt.Sprintf("Content-Type %s; esperado application/json", oauth.ClientValue(contentType)))
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRegisterBodyBytes))
	var req oauth.RegistrationRequest
	if err := decoder.Decode(&req); err != nil {
		return oauth.RegistrationRequest{}, invalidClientMetadata(fmt.Sprintf("corpo ilegivel ou acima de %d bytes; esperado um objeto JSON com os metadados do cliente", maxRegisterBodyBytes))
	}
	if !errors.Is(decoder.Decode(&struct{}{}), io.EOF) {
		return oauth.RegistrationRequest{}, invalidClientMetadata("dados depois do objeto JSON; esperado um unico objeto")
	}
	return req, nil
}

func invalidClientMetadata(description string) *oauth.ProtocolError {
	return &oauth.ProtocolError{Code: oauthErrInvalidClientMetadata, Description: description}
}

// writeRegistrationError responde a recusa temporária com 429 e Retry-After em
// segundos inteiros, arredondados para cima, e o resto como os outros endpoints.
func writeRegistrationError(w http.ResponseWriter, err error) {
	var protoErr *oauth.ProtocolError
	if !errors.As(err, &protoErr) || protoErr.RetryAfter <= 0 {
		writeOAuthServiceError(w, "registrar cliente oauth", err)
		return
	}
	slog.Warn("registro dinamico de cliente oauth recusado", "code", protoErr.Code, "error", protoErr.Unwrap())
	w.Header().Set("Retry-After", strconv.FormatFloat(math.Ceil(protoErr.RetryAfter.Seconds()), 'f', 0, 64))
	writeJSON(w, http.StatusTooManyRequests, oauthErrorResponse{Error: protoErr.Code, ErrorDescription: protoErr.Description})
}
