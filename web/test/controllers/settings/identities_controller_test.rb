require "test_helper"

# Contas conectadas em Configurações > Conta: conectar um provedor à conta logada, listar e desconectar, sem deixar a
# conta sem forma de entrar.
module Settings
  class IdentitiesControllerTest < ActionDispatch::IntegrationTest
    GITHUB = FakeMarkuppExternalLogin::DEFAULT_PROVIDERS

    def connect_github(code = "codigo-novo")
      post external_login_path("github"), params: { purpose: "link" }
      state = Rack::Utils.parse_query(URI(response.location).query).fetch("state")
      get external_login_callback_path("github"), params: { code:, state: }
    end

    test "the account settings offer to connect each provider" do
      use_fake_api(login_providers: GITHUB)
      sign_in_as

      get settings_account_path

      assert_select "h2", "Connected accounts"
      assert_select "form[action=?] input[name=purpose][value=link]", external_login_path("github")
      assert_select "form[action=?] button", external_login_path("github"), text: "Connect GitHub"
    end

    test "a server without providers shows no connected accounts section" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "h2", text: "Connected accounts", count: 0
    end

    test "connecting links the identity to the account and lists it" do
      use_fake_api(login_providers: GITHUB)
      sign_in_as

      connect_github

      assert_redirected_to settings_account_path
      assert_equal "GitHub connected.", flash[:notice]
      follow_redirect!
      assert_select "form[action=?][method=post] input[name=_method][value=delete]", settings_identity_path("identity-1")
      assert_select "form[action=?] button", settings_identity_path("identity-1"), text: "Disconnect GitHub"
      assert_select "form[action=?]", external_login_path("github"), count: 0
    end

    test "connecting an identity of another account is refused" do
      use_fake_api(login_providers: GITHUB, accounts: [ *FakeMarkuppAccounts::DEFAULT_ACCOUNTS, { email: "caio@markupp.dev", password: "outra" } ])
        .link_identity!("codigo-ana", FakeMarkuppAccounts::DEFAULT_EMAIL)
      sign_in_as("caio@markupp.dev", password: "outra")

      connect_github("codigo-ana")

      assert_redirected_to settings_account_path
      assert_equal "This GitHub identity already belongs to an account.", flash[:alert]
    end

    test "disconnecting removes the identity" do
      use_fake_api(login_providers: GITHUB)
      sign_in_as
      connect_github

      delete settings_identity_path("identity-1")

      assert_redirected_to settings_account_path
      assert_equal "GitHub disconnected.", flash[:notice]
      follow_redirect!
      assert_select "form[action=?]", settings_identity_path("identity-1"), count: 0
    end

    test "the last way to log in is not disconnected" do
      use_fake_api(login_providers: GITHUB)
      post external_login_path("github"), params: { purpose: "signup" }
      state = Rack::Utils.parse_query(URI(response.location).query).fetch("state")
      get external_login_callback_path("github"), params: { code: "codigo-novo", state: }

      delete settings_identity_path("identity-1")

      assert_redirected_to settings_account_path
      assert_equal "This is the only way left to log in to your account. Connect another one first.", flash[:alert]
    end

    test "an account without password hides the password change" do
      use_fake_api(login_providers: GITHUB)
      post external_login_path("github"), params: { purpose: "signup" }
      state = Rack::Utils.parse_query(URI(response.location).query).fetch("state")
      get external_login_callback_path("github"), params: { code: "codigo-novo", state: }

      get settings_account_path

      assert_select "form[action=?]", password_change_path, count: 0
    end
  end
end
