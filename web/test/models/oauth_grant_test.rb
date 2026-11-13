require "test_helper"

class OAuthGrantTest < ActiveSupport::TestCase
  ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
  BIA = "bia@markupp.dev"
  PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
  ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: BIA, password: PASSWORD } ].freeze
  AUTHORIZATIONS = [
    { id: "auth-1", client_name: "markupp-mcp (terminal)", scopes: %w[notas:leitura notas:escrita], created_at: "2026-10-16T12:00:00Z" },
    { id: "auth-2", client_name: "markupp-cli (terminal)", created_at: "2026-10-20T09:30:00Z" },
    { id: "auth-3", account: BIA, client_name: "Claude" }
  ].freeze

  setup do
    @fake_api = FakeMarkuppApi.new(accounts: ACCOUNTS, authorizations: AUTHORIZATIONS)
  end

  test "all builds the authorizations of the account with client, scopes and authorization date" do
    assert_equal [ OAuthGrant.new(id: "auth-1", client_name: "markupp-mcp (terminal)", scopes: %w[notas:leitura notas:escrita],
                                  created_at: Time.utc(2026, 10, 16, 12)),
                   OAuthGrant.new(id: "auth-2", client_name: "markupp-cli (terminal)", scopes: %w[notas:leitura],
                                  created_at: Time.utc(2026, 10, 20, 9, 30)) ],
                 OAuthGrant.all(api_as(ANA))
  end

  test "revoke drops the authorization from the account" do
    api = api_as(ANA)

    OAuthGrant.revoke(api, "auth-1")

    assert_equal %w[auth-2], OAuthGrant.all(api).map(&:id)
  end

  test "revoke raises AuthorizationNotFound for one that is gone or belongs to another account" do
    api = api_as(ANA)

    assert_raises(MarkuppApi::AuthorizationNotFound) { OAuthGrant.revoke(api, "auth-9") }
    assert_raises(MarkuppApi::AuthorizationNotFound) { OAuthGrant.revoke(api, "auth-3") }
  end

  private
    def api_as(email)
      pair = @fake_api.post("/auth/login", email:, password: PASSWORD)
      @fake_api.as(access_token: pair.fetch("access_token"))
    end
end
