require "application_system_test_case"

# Login externo da instância pelo teclado: a paleta leva à seção, o preset do Google preenche o começo do formulário e
# apagar pede a confirmação do painel com as contas que perdem essa forma de entrar.
class InstanceLoginProvidersTest < ApplicationSystemTestCase
  GITHUB = { id: "github", name: "GitHub", client_id: "gh-client", client_secret: "gh-secret", linked_accounts: 2 }.freeze

  setup do
    use_fake_api(accounts: [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ],
                 instance_login_providers: { providers: [ GITHUB ] })
    sign_in_through_form
  end

  test "the palette opens the section and the Google preset adds a provider" do
    visit settings_account_path
    find("body").send_keys([ :control, :shift, "p" ])
    find("#command_palette input[role=combobox]").send_keys("Instance external login", :enter)

    assert_current_path settings_instance_login_providers_path
    within("#new_login_provider") { click_on "Google" }
    within("#new_login_provider") do
      assert_field "Issuer", with: "https://accounts.google.com"
      fill_in "Client ID", with: "g-client"
      fill_in "Client secret", with: "g-secret"
      find_field("Client secret").send_keys(:enter)
    end

    assert_selector "[role=status]", text: "Provider added."
    assert_selector "#login_provider_google pre", text: "/login-providers/google/callback"
  end

  test "deleting a provider asks the panel confirmation with the linked accounts" do
    visit settings_instance_login_providers_path

    accept_panel_confirm("Delete GitHub? 2 accounts lose this way to sign in. This cannot be undone.") do
      within("#login_provider_github") { click_on "Delete" }
    end

    assert_selector "[role=status]", text: "Provider deleted. 2 accounts lost this way to sign in."
    assert_no_selector "#login_provider_github"
  end
end
