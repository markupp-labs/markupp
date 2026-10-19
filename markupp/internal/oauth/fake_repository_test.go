package oauth_test

import (
	"context"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// fakeOAuthRepository guarda em memória o que o Service pede ao repositório e devolve o
// que cada teste configurou. Os campos *Err forçam a falha do método correspondente.
type fakeOAuthRepository struct {
	recordedEvents []oauth.AuditEvent
	recordErr      error
	clients        map[string]oauth.Client
	requests       map[string]oauth.AuthorizationRequest
	authorizations map[string]oauth.Authorization
	redeemed       oauth.RedeemedCode
	consumed       oauth.TokenGrant
	found          oauth.TokenGrant
	summaries      []oauth.AuthorizationSummary
	// freshCIMD são os clientes CIMD com documento em cache ainda válido.
	freshCIMD map[string]oauth.Client

	getClientErr, createErr, approveErr, activeErr, redeemErr, saveErr              error
	consumeErr, saveRefreshedErr, revokeAuditedErr, findErr, listErr, revokeAuthErr error
	getFreshCIMDErr, saveCIMDErr, registerDCRErr                                    error

	created     []oauth.AuthorizationRequest
	pendingNow  []time.Time
	approvals   []oauth.Approval
	denials     []oauth.Denial
	redeemCalls []string
	redeemNow   []time.Time
	// redeemWindows guarda a janela de reuso de cada chamada a RedeemCode.
	redeemWindows []time.Duration
	saved         [][]oauth.IssuedToken
	// savedRequests guarda o pedido de cada chamada a SaveCodeTokens.
	savedRequests []string

	consumeCalls   []string
	consumeNow     []time.Time
	refreshParents []string
	refreshed      [][]oauth.IssuedToken
	refreshEvents  []oauth.AuditEvent
	auditedRevokes []auditedRevoke
	findCalls      []string
	listCalls      []string
	revokedAuths   []revokedAuthorization
	getClientCalls []string
	freshCIMDCalls []freshCIMDLookup
	savedCIMD      []savedCIMDClient
	// dcrRegistrations guarda cada chamada a RegisterDCRClient.
	dcrRegistrations []oauth.DCRRegistration
}

// freshCIMDLookup é uma chamada a GetFreshCIMDClient.
type freshCIMDLookup struct {
	ID  string
	Now time.Time
}

// savedCIMDClient é uma chamada a SaveCIMDClient.
type savedCIMDClient struct {
	Client    oauth.Client
	ExpiresAt time.Time
	Now       time.Time
}

// auditedRevoke é uma chamada a RevokeAuthorizationTokensAudited.
type auditedRevoke struct {
	AuthorizationID string
	TriggerHash     string
	Expiry          oauth.TriggerExpiry
	Event           oauth.AuditEvent
}

// revokedAuthorization é uma chamada a RevokeAuthorization.
type revokedAuthorization struct {
	UserID          string
	AuthorizationID string
	Event           oauth.AuditEvent
}

func newFakeOAuthRepository() *fakeOAuthRepository {
	return &fakeOAuthRepository{
		clients:        map[string]oauth.Client{},
		requests:       map[string]oauth.AuthorizationRequest{},
		authorizations: map[string]oauth.Authorization{},
		freshCIMD:      map[string]oauth.Client{},
	}
}

func authorizationKey(userID, clientID string) string {
	return userID + "|" + clientID
}

func (f *fakeOAuthRepository) GetClient(_ context.Context, id string) (oauth.Client, error) {
	f.getClientCalls = append(f.getClientCalls, id)
	if f.getClientErr != nil {
		return oauth.Client{}, f.getClientErr
	}
	client, ok := f.clients[id]
	if !ok {
		return oauth.Client{}, oauth.ErrClientNotFound
	}
	return client, nil
}

func (f *fakeOAuthRepository) CreateRequest(_ context.Context, req oauth.AuthorizationRequest) error {
	f.created = append(f.created, req)
	return f.createErr
}

func (f *fakeOAuthRepository) GetPendingRequest(_ context.Context, id string, now time.Time) (oauth.AuthorizationRequest, error) {
	f.pendingNow = append(f.pendingNow, now)
	req, ok := f.requests[id]
	if !ok {
		return oauth.AuthorizationRequest{}, oauth.ErrRequestNotFound
	}
	return req, nil
}

func (f *fakeOAuthRepository) ApproveRequest(_ context.Context, approval oauth.Approval) (oauth.AuthorizationRequest, error) {
	f.approvals = append(f.approvals, approval)
	if f.approveErr != nil {
		return oauth.AuthorizationRequest{}, f.approveErr
	}
	req, ok := f.requests[approval.RequestID]
	if !ok {
		return oauth.AuthorizationRequest{}, oauth.ErrRequestNotFound
	}
	return req, nil
}

func (f *fakeOAuthRepository) DenyRequest(_ context.Context, denial oauth.Denial) (oauth.AuthorizationRequest, error) {
	f.denials = append(f.denials, denial)
	req, ok := f.requests[denial.RequestID]
	if !ok {
		return oauth.AuthorizationRequest{}, oauth.ErrRequestNotFound
	}
	return req, nil
}

func (f *fakeOAuthRepository) ActiveAuthorization(_ context.Context, userID, clientID string) (oauth.Authorization, error) {
	if f.activeErr != nil {
		return oauth.Authorization{}, f.activeErr
	}
	authorization, ok := f.authorizations[authorizationKey(userID, clientID)]
	if !ok {
		return oauth.Authorization{}, oauth.ErrAuthorizationNotFound
	}
	return authorization, nil
}

func (f *fakeOAuthRepository) RedeemCode(_ context.Context, codeHash string, now time.Time, reuseWindow time.Duration) (oauth.RedeemedCode, error) {
	f.redeemCalls = append(f.redeemCalls, codeHash)
	f.redeemNow = append(f.redeemNow, now)
	f.redeemWindows = append(f.redeemWindows, reuseWindow)
	return f.redeemed, f.redeemErr
}

func (f *fakeOAuthRepository) SaveCodeTokens(_ context.Context, requestID string, tokens []oauth.IssuedToken) error {
	f.savedRequests = append(f.savedRequests, requestID)
	f.saved = append(f.saved, tokens)
	return f.saveErr
}

func (f *fakeOAuthRepository) ConsumeRefreshToken(_ context.Context, tokenHash string, now time.Time) (oauth.TokenGrant, error) {
	f.consumeCalls = append(f.consumeCalls, tokenHash)
	f.consumeNow = append(f.consumeNow, now)
	return f.consumed, f.consumeErr
}

func (f *fakeOAuthRepository) SaveRefreshedTokens(_ context.Context, parentHash string, tokens []oauth.IssuedToken, event oauth.AuditEvent) error {
	f.refreshParents = append(f.refreshParents, parentHash)
	f.refreshed = append(f.refreshed, tokens)
	f.refreshEvents = append(f.refreshEvents, event)
	return f.saveRefreshedErr
}

func (f *fakeOAuthRepository) RevokeAuthorizationTokensAudited(_ context.Context, authorizationID, triggerHash string, expiry oauth.TriggerExpiry, event oauth.AuditEvent) error {
	f.auditedRevokes = append(f.auditedRevokes, auditedRevoke{
		AuthorizationID: authorizationID, TriggerHash: triggerHash, Expiry: expiry, Event: event,
	})
	return f.revokeAuditedErr
}

// FindToken devolve found, ou ErrTokenNotFound enquanto found não foi configurado.
func (f *fakeOAuthRepository) FindToken(_ context.Context, tokenHash string) (oauth.TokenGrant, error) {
	f.findCalls = append(f.findCalls, tokenHash)
	if f.findErr == nil && f.found.Authorization.ID == "" {
		return oauth.TokenGrant{}, oauth.ErrTokenNotFound
	}
	return f.found, f.findErr
}

func (f *fakeOAuthRepository) ListActiveAuthorizations(_ context.Context, userID string) ([]oauth.AuthorizationSummary, error) {
	f.listCalls = append(f.listCalls, userID)
	return f.summaries, f.listErr
}

func (f *fakeOAuthRepository) RevokeAuthorization(_ context.Context, userID, authorizationID string, event oauth.AuditEvent) error {
	f.revokedAuths = append(f.revokedAuths, revokedAuthorization{UserID: userID, AuthorizationID: authorizationID, Event: event})
	return f.revokeAuthErr
}

func (f *fakeOAuthRepository) RecordAuditEvent(_ context.Context, event oauth.AuditEvent) error {
	f.recordedEvents = append(f.recordedEvents, event)
	return f.recordErr
}

func (f *fakeOAuthRepository) GetFreshCIMDClient(_ context.Context, id string, now time.Time) (oauth.Client, error) {
	f.freshCIMDCalls = append(f.freshCIMDCalls, freshCIMDLookup{ID: id, Now: now})
	if f.getFreshCIMDErr != nil {
		return oauth.Client{}, f.getFreshCIMDErr
	}
	client, ok := f.freshCIMD[id]
	if !ok {
		return oauth.Client{}, oauth.ErrClientNotFound
	}
	return client, nil
}

func (f *fakeOAuthRepository) RegisterDCRClient(_ context.Context, reg oauth.DCRRegistration) error {
	f.dcrRegistrations = append(f.dcrRegistrations, reg)
	return f.registerDCRErr
}

func (f *fakeOAuthRepository) SaveCIMDClient(_ context.Context, client oauth.Client, expiresAt, now time.Time) error {
	f.savedCIMD = append(f.savedCIMD, savedCIMDClient{Client: client, ExpiresAt: expiresAt, Now: now})
	return f.saveCIMDErr
}
