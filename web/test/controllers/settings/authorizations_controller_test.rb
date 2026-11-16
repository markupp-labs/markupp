require "test_helper"

# Configurações da conta, seção Autorizações: lista os clientes que a conta autorizou e revoga cada um.
module Settings
  class AuthorizationsControllerTest < ActionDispatch::IntegrationTest
    BIA = "bia@markupp.dev"
    ACCOUNTS = [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD },
                 { email: BIA, password: FakeMarkuppAccounts::DEFAULT_PASSWORD } ].freeze
    AUTHORIZATIONS = [
      { id: "auth-1", client_name: "markupp-mcp (terminal)", scopes: %w[notas:leitura notas:escrita], created_at: "2026-10-16T12:00:00Z" },
      { id: "auth-2", client_name: "markupp-cli (terminal)", scopes: %w[notas:leitura administracao], created_at: "2026-10-20T09:30:00Z" },
      { id: "auth-3", account: BIA, client_name: "Claude" }
    ].freeze

    test "the authorizations section is current in the settings nav, right after account" do
      use_fake_api
      sign_in_as

      get settings_authorizations_path

      assert_response :success
      assert_select "main h1", "Authorizations"
      assert_select "main nav a[href=?][aria-current=page]", settings_authorizations_path, text: "Authorizations"
      assert_equal [ settings_account_path, settings_authorizations_path ], css_select("main nav a").map { it["href"] }.first(2)
    end

    test "lists the authorizations of the account with client, date and scopes" do
      use_fake_api(accounts: ACCOUNTS, authorizations: AUTHORIZATIONS)
      sign_in_as

      get settings_authorizations_path

      assert_equal [ "markupp-mcp (terminal)", "markupp-cli (terminal)" ], css_select("#oauth_grants li[data-oauth-grant] .client-name").map { it.text.strip }
      assert_select "#oauth_grants li[data-oauth-grant=auth-1]", text: /Oct 16.*Read your notes.*Create, edit and delete your notes/m
      assert_select "#oauth_grants li[data-oauth-grant=auth-2]", text: /Oct 20.*Read your notes.*Manage your tenants, members and invites/m
      assert_select "#oauth_grants li", text: /Claude/, count: 0
    end

    test "each authorization has a revoke button that asks first" do
      use_fake_api(authorizations: AUTHORIZATIONS)
      sign_in_as

      get settings_authorizations_path

      assert_select "#oauth_grants li[data-oauth-grant=auth-1][data-context-menu-target=item][tabindex='0']" do
        assert_select "form[action=?] button.btn-danger[data-context-menu-delete][data-confirm-button=Revoke][data-turbo-confirm=?]",
                      settings_authorization_path("auth-1"), "Revoke the authorization for markupp-mcp (terminal)?", text: "Revoke"
      end
      assert_select "#oauth_grants li[data-oauth-grant=auth-2][tabindex='-1']"
    end

    test "says where authorizations come from when there are none" do
      use_fake_api
      sign_in_as

      get settings_authorizations_path

      assert_select "main", text: /No authorizations yet\. Clients such as markupp-mcp and markupp-cli show up here once you log in with them\./
      assert_select "#oauth_grants", count: 0
    end

    test "revoking an authorization drops it" do
      use_fake_api(authorizations: AUTHORIZATIONS)
      sign_in_as

      delete settings_authorization_path("auth-1")

      assert_redirected_to settings_authorizations_path
      follow_redirect!
      assert_select "[role=status]", "Authorization revoked."
      assert_select "#oauth_grants li", text: /markupp-mcp/, count: 0
      assert_select "#oauth_grants li", text: /markupp-cli/
    end

    test "revoking an authorization that is gone says so" do
      use_fake_api(accounts: ACCOUNTS, authorizations: AUTHORIZATIONS)
      sign_in_as

      delete settings_authorization_path("auth-3")

      assert_redirected_to settings_authorizations_path
      follow_redirect!
      assert_select "[role=alert]", "That authorization is no longer active."
    end

    test "works without a tenant" do
      use_fake_api(tenants: [], authorizations: AUTHORIZATIONS)
      sign_in_as

      get settings_authorizations_path

      assert_response :success
      assert_select "#oauth_grants li[data-oauth-grant]", count: 2
    end

    test "answers bad gateway when the API is unreachable" do
      api = use_fake_api
      sign_in_as
      api.cut_connection!

      get settings_authorizations_path

      assert_response :bad_gateway
      assert_select "main", text: /Can't reach the markupp server/
    end
  end
end
