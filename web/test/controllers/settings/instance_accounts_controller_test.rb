require "test_helper"

# Configurações da instância, seção Contas (ADR-0061): o administrador da instância lista as contas, vê as sessões e os apps
# de cada uma, bloqueia, desbloqueia e apaga, sabendo antes o que some junto.
module Settings
  class InstanceAccountsControllerTest < ActionDispatch::IntegrationTest
    ADMIN_ACCOUNTS = [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ].freeze
    FIREFOX = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0".freeze
    BIA = { id: "u-bia", email: "bia@exemplo.org", login_methods: %w[github local], last_access_at: "2026-11-25T11:00:00Z",
            sessions: [ { id: "s1", ip: "203.0.113.7", user_agent: FIREFOX, created_at: "2026-11-20T12:00:00Z",
                          last_used_at: "2026-11-25T11:00:00Z", current: false } ],
            authorizations: [ { id: "a1", client_id: "c1", client_name: "Claude", scopes: %w[notas:leitura], created_at: "2026-11-01T12:00:00Z",
                                updated_at: "2026-11-01T12:00:00Z", ip: "198.51.100.9", user_agent: "Claude-User/1.0",
                                last_used_at: "2026-11-25T10:00:00Z" } ],
            sole_owner_tenants: [ { id: "t1", name: "Casa", notes: 12 } ] }.freeze
    CAIO = { id: "u-caio", email: "caio@exemplo.org", blocked: true }.freeze

    def sign_in_admin(**scenario)
      fake = use_fake_api(accounts: ADMIN_ACCOUNTS, instance_access: scenario)
      sign_in_as
      fake
    end

    test "an instance admin finds the accounts section in the instance group" do
      sign_in_admin(accounts: [ BIA ])

      get settings_instance_accounts_path

      assert_response :success
      assert_select "main h1", "Accounts"
      assert_select "main nav a[href=?][aria-current=page]", settings_instance_accounts_path, text: "Accounts"
    end

    test "an account that does not administer the instance is sent away" do
      use_fake_api
      sign_in_as

      get settings_instance_accounts_path

      assert_redirected_to root_path
    end

    test "the list shows email, how they sign in, last access and the blocked ones" do
      sign_in_admin(accounts: [ BIA, CAIO ])

      get settings_instance_accounts_path

      assert_select "#instance_accounts li[data-account=u-bia]", text: /bia@exemplo\.org.*github, local/m do
        assert_select "a[href=?]", settings_instance_account_path("u-bia")
      end
      assert_select "#instance_accounts li[data-account=u-caio]", text: /caio@exemplo\.org.*Never.*Blocked/m
    end

    test "the search and the status filter go to the API" do
      fake = sign_in_admin(accounts: [ BIA, CAIO ])

      get settings_instance_accounts_path(q: "caio", status: "blocked")

      assert_select "#instance_accounts li[data-account]", count: 1
      assert_select "form[role=search] input[name=q][value=caio]"
      assert_includes fake.instance_access.queries, "q=caio&status=blocked&limit=50&offset=0"
    end

    test "a full page links to the next one" do
      accounts = Array.new(51) { |i| { id: "u#{i}", email: "u#{i}@exemplo.org" } }
      sign_in_admin(accounts:)

      get settings_instance_accounts_path

      assert_select "#instance_accounts li[data-account]", count: 50
      assert_select "a[rel=next][href=?]", settings_instance_accounts_path(offset: 50)
    end

    test "the account page shows the sessions, the apps and what goes with a deletion" do
      sign_in_admin(accounts: [ BIA ])

      get settings_instance_account_path("u-bia")

      assert_select "main h1", "bia@exemplo.org"
      assert_select "#account_sessions li[data-session=s1]", text: /Firefox on Linux.*203\.0\.113\.7/m do
        assert_select "form[action=?] button", settings_instance_account_session_path("u-bia", "s1"), text: "End"
      end
      assert_select "#account_authorizations li[data-authorization=a1]", text: /Claude.*Claude-User.*198\.51\.100\.9/m do
        assert_select "form[action=?] button", settings_instance_account_authorization_path("u-bia", "a1"), text: "Revoke"
      end
      assert_select "form[action=?] button[data-turbo-confirm=?]", settings_instance_account_path("u-bia"),
                    "Delete bia@exemplo.org? Its sessions and apps go too, and so do 1 tenant and 12 notes where it is the only owner.",
                    text: "Delete account"
    end

    test "blocking and unblocking go back to the account with a notice" do
      fake = sign_in_admin(accounts: [ BIA ])

      patch settings_instance_account_path("u-bia"), params: { account: { blocked: "1" } }
      assert_redirected_to settings_instance_account_path("u-bia")
      assert_equal "Account blocked. It leaves within minutes.", flash[:notice]
      assert fake.instance_access.account("u-bia")["blocked"]

      patch settings_instance_account_path("u-bia"), params: { account: { blocked: "0" } }
      assert_equal "Account unblocked.", flash[:notice]
      assert_not fake.instance_access.account("u-bia")["blocked"]
    end

    test "the blocked account page offers unblock" do
      sign_in_admin(accounts: [ CAIO ])

      get settings_instance_account_path("u-caio")

      assert_select "#account_status", text: /Blocked/
      assert_select "form[action=?] button", settings_instance_account_path("u-caio"), text: "Unblock"
    end

    test "deleting goes back to the list" do
      fake = sign_in_admin(accounts: [ BIA ])

      delete settings_instance_account_path("u-bia")

      assert_redirected_to settings_instance_accounts_path
      assert_equal "Account deleted.", flash[:notice]
      assert_nil fake.instance_access.account("u-bia")
    end

    test "the admin can't block or delete their own account" do
      sign_in_admin(accounts: [ { id: "account-1", email: FakeMarkuppAccounts::DEFAULT_EMAIL } ])

      delete settings_instance_account_path("account-1")

      assert_redirected_to settings_instance_account_path("account-1")
      assert_equal "You can't block or delete your own account.", flash[:alert]
    end

    test "ending a session and revoking an app of the account" do
      fake = sign_in_admin(accounts: [ BIA ])

      delete settings_instance_account_session_path("u-bia", "s1")
      assert_redirected_to settings_instance_account_path("u-bia")
      assert_equal "Session ended. That device signs out within minutes.", flash[:notice]

      delete settings_instance_account_authorization_path("u-bia", "a1")
      assert_equal "Authorization revoked.", flash[:notice]
      assert_empty fake.instance_access.account("u-bia").values_at("sessions", "authorizations").flatten
    end

    test "an account that no longer exists goes back to the list" do
      sign_in_admin

      get settings_instance_account_path("u-nenhuma")

      assert_redirected_to settings_instance_accounts_path
      assert_equal "That account no longer exists.", flash[:alert]
    end
  end
end
