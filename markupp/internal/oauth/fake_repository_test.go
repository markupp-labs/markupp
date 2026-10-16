package oauth_test

import (
	"context"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// fakeOAuthRepository guarda em memória o que o Service pede ao repositório e devolve o
// que cada teste configurou. Os campos *Err forçam a falha do método correspondente.
type fakeOAuthRepository struct {
	clients        map[string]oauth.Client
	requests       map[string]oauth.AuthorizationRequest
	authorizations map[string]oauth.Authorization
	redeemed       oauth.RedeemedCode

	createErr, approveErr, activeErr, redeemErr, saveErr, revokeErr error

	created     []oauth.AuthorizationRequest
	pendingNow  []time.Time
	approvals   []oauth.Approval
	denials     []oauth.Denial
	redeemCalls []string
	redeemNow   []time.Time
	saved       [][]oauth.IssuedToken
	revoked     []string
}

func newFakeOAuthRepository() *fakeOAuthRepository {
	return &fakeOAuthRepository{
		clients:        map[string]oauth.Client{},
		requests:       map[string]oauth.AuthorizationRequest{},
		authorizations: map[string]oauth.Authorization{},
	}
}

func authorizationKey(userID, clientID string) string {
	return userID + "|" + clientID
}

func (f *fakeOAuthRepository) GetClient(_ context.Context, id string) (oauth.Client, error) {
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

func (f *fakeOAuthRepository) RedeemCode(_ context.Context, codeHash string, now time.Time) (oauth.RedeemedCode, error) {
	f.redeemCalls = append(f.redeemCalls, codeHash)
	f.redeemNow = append(f.redeemNow, now)
	return f.redeemed, f.redeemErr
}

func (f *fakeOAuthRepository) SaveTokens(_ context.Context, tokens []oauth.IssuedToken) error {
	f.saved = append(f.saved, tokens)
	return f.saveErr
}

func (f *fakeOAuthRepository) RevokeAuthorizationTokens(_ context.Context, authorizationID string) error {
	f.revoked = append(f.revoked, authorizationID)
	return f.revokeErr
}
