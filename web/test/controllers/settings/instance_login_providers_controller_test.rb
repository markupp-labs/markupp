require "test_helper"

# Configurações da instância, seção Login externo: só o administrador da instância lista, cria, edita, liga, desliga e
# apaga os provedores. O segredo nunca volta à página; a lista fixada por quem hospeda fica só para leitura.
module Settings
  class InstanceLoginProvidersControllerTest < ActionDispatch::IntegrationTest
    ADMIN_ACCOUNTS = [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ].freeze
    GITHUB = { id: "github", name: "GitHub", client_id: "gh-client", client_secret: "gh-secret", linked_accounts: 3 }.freeze
    GOOGLE = { id: "google", name: "Google", issuer: "https://accounts.google.com", client_id: "g-client", client_secret: "g-secret" }.freeze

    def sign_in_admin(**scenario)
      fake = use_fake_api(accounts: ADMIN_ACCOUNTS, instance_login_providers: scenario)
      sign_in_as
      fake
    end

    def new_provider(**fields)
      { id: "google", name: "Google", issuer: "https://accounts.google.com", client_id: "g-client", client_secret: "g-secret", enabled: "1" }.merge(fields)
    end

    test "an instance admin finds the external login section in the instance group" do
      sign_in_admin

      get settings_instance_login_providers_path

      assert_response :success
      assert_select "main h1", "External login"
      assert_select "main nav a[href=?][aria-current=page]", settings_instance_login_providers_path, text: "External login"
    end

    test "an account that does not administer the instance has no external login section and is sent away" do
      use_fake_api
      sign_in_as

      get settings_account_path
      assert_select "main nav a[href=?]", settings_instance_login_providers_path, count: 0

      get settings_instance_login_providers_path
      assert_redirected_to root_path
    end

    test "each provider shows its name, id, issuer, state, linked accounts and callback URL, never the secret" do
      sign_in_admin(providers: [ GITHUB, GOOGLE.merge(enabled: false) ])

      get settings_instance_login_providers_path

      assert_select "#login_provider_github" do
        assert_select "h2", /GitHub\s+github/
        assert_select "dd", "GitHub"
        assert_select "dd", "On"
        assert_select "dd", "3"
        assert_select "pre", "https://markupp.test/v1/auth/login-providers/github/callback"
        assert_select "input[type=password][name=?]:not([value])", "login_provider[client_secret]"
      end
      assert_select "#login_provider_google dd", "https://accounts.google.com"
      assert_select "#login_provider_google dd", "Off"
      assert_select "main", text: /A secret is set/
      assert_no_match(/gh-secret|g-secret/, response.body)
    end

    test "creating a provider sends its fields and says to register the callback URL" do
      fake = sign_in_admin

      post settings_instance_login_providers_path, params: { login_provider: new_provider }

      assert_redirected_to settings_instance_login_providers_path
      follow_redirect!
      assert_select "[role=status]", "Provider added. Register the callback URL at the provider."
      assert_equal [ :post, "google", { id: "google", name: "Google", issuer: "https://accounts.google.com", client_id: "g-client",
                                        client_secret: "g-secret", enabled: true } ], fake.instance_login_providers_writes.last
      assert_select "#login_provider_google"
    end

    test "a refused provider stays on the form with what was typed, except the secret" do
      sign_in_admin

      post settings_instance_login_providers_path, params: { login_provider: new_provider(id: "Google!", client_secret: "typed-secret") }

      assert_response :unprocessable_entity
      assert_select "main [role=alert]", /Check the fields/
      assert_select "#new_login_provider input[name=?][value=?]", "login_provider[id]", "Google!"
      assert_no_match(/typed-secret/, response.body)
    end

    test "an id already in use is refused with the reason" do
      sign_in_admin(providers: [ GOOGLE ])

      post settings_instance_login_providers_path, params: { login_provider: new_provider }

      assert_response :unprocessable_entity
      assert_select "main [role=alert]", "There is already a provider with this ID."
    end

    test "the GitHub and Google presets fill the id, name and issuer of the new provider" do
      sign_in_admin

      get settings_instance_login_providers_path(preset: "google")

      assert_select "#new_login_provider" do
        assert_select "input[name=?][value=google]", "login_provider[id]"
        assert_select "input[name=?][value=Google]", "login_provider[name]"
        assert_select "input[name=?][value=?]", "login_provider[issuer]", "https://accounts.google.com"
        assert_select "a[href=?]", settings_instance_login_providers_path(preset: "github", anchor: "new_login_provider")
      end
    end

    test "editing with an empty secret keeps the stored one and can turn the provider off" do
      fake = sign_in_admin(providers: [ GOOGLE ])

      patch settings_instance_login_provider_path("google"),
            params: { login_provider: { name: "Google Workspace", issuer: "https://accounts.google.com", client_id: "g-client", client_secret: "", enabled: "0" } }

      assert_redirected_to settings_instance_login_providers_path
      verb, id, body = fake.instance_login_providers_writes.last
      assert_equal [ :put, "google" ], [ verb, id ]
      assert_not body.key?(:client_secret)
      assert_not body[:enabled]
      follow_redirect!
      assert_select "#login_provider_google dd", "Off"
      assert_select "#login_provider_google", text: /A secret is set/
    end

    test "changing the issuer of a provider with linked accounts is refused and keeps what was typed" do
      sign_in_admin(providers: [ GOOGLE.merge(linked_accounts: 2) ])

      patch settings_instance_login_provider_path("google"),
            params: { login_provider: { name: "Google", issuer: "https://other.example", client_id: "g-client", enabled: "1" } }

      assert_response :unprocessable_entity
      assert_select "main [role=alert]", "The issuer can't change while accounts sign in with this provider."
      assert_select "#login_provider_google input[name=?][value=?]", "login_provider[issuer]", "https://other.example"
    end

    test "deleting asks the panel confirmation with the accounts that lose this way to sign in" do
      fake = sign_in_admin(providers: [ GITHUB ])

      get settings_instance_login_providers_path
      assert_select "#login_provider_github button[data-turbo-confirm=?]",
                    "Delete GitHub? 3 accounts lose this way to sign in. This cannot be undone."

      delete settings_instance_login_provider_path("github")

      assert_redirected_to settings_instance_login_providers_path
      follow_redirect!
      assert_select "[role=status]", "Provider deleted. 3 accounts lost this way to sign in."
      assert_equal [ :delete, "github", nil ], fake.instance_login_providers_writes.last
      assert_select "#login_provider_github", count: 0
    end

    test "a provider that no longer exists is reported on the list" do
      sign_in_admin

      delete settings_instance_login_provider_path("gone")

      assert_redirected_to settings_instance_login_providers_path
      follow_redirect!
      assert_select "[role=alert]", "This provider no longer exists on this instance."
    end

    test "a list fixed by whoever hosts the instance is read-only, without naming settings" do
      sign_in_admin(locked: true, providers: [ GITHUB ])

      get settings_instance_login_providers_path

      assert_select "#login_providers_locked", "Whoever hosts this instance fixed this list, so it can't be changed here."
      assert_select "#login_provider_github pre", /callback/
      assert_select "#login_provider_github form", count: 0
      assert_select "#new_login_provider", count: 0
      assert_no_match(/MARKUPP_/, response.body)
    end

    test "the palette leads an instance admin to the external login section" do
      sign_in_admin

      get settings_account_path

      assert_select "#command_palette_options [role=option][data-command-href=?]", settings_instance_login_providers_path,
                    text: /Instance external login/
    end
  end
end
