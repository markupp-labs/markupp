require "application_system_test_case"

# Seção Autorizações das configurações: revogar pede a confirmação do painel, pelo botão ou por Delete na linha com o foco.
class AuthorizationsTest < ApplicationSystemTestCase
  AUTHORIZATIONS = [ { id: "auth-1", client_name: "markupp-mcp (terminal)" }, { id: "auth-2", client_name: "markupp-cli (terminal)" } ].freeze

  setup do
    use_fake_api(authorizations: AUTHORIZATIONS)
    sign_in_through_form
  end

  test "revokes an authorization after confirming and keeps the other" do
    visit settings_authorizations_path
    within("#oauth_grants li[data-oauth-grant=auth-1]") do
      accept_panel_confirm("Revoke the authorization for markupp-mcp (terminal)?", button: "Revoke") { click_on "Revoke" }
    end

    assert_text "Authorization revoked."
    assert_no_selector "#oauth_grants", text: "markupp-mcp"
    assert_selector "#oauth_grants", text: "markupp-cli"
  end

  test "j moves to the next authorization and delete asks to revoke the one in focus" do
    visit settings_authorizations_path
    execute_script("document.querySelector(\"#oauth_grants li[tabindex='0']\").focus()")
    assert_selector "#oauth_grants li:focus", text: "markupp-mcp"

    send_keys "j"
    assert_selector "#oauth_grants li:focus", text: "markupp-cli"
    accept_panel_confirm("Revoke the authorization for markupp-cli (terminal)?", button: "Revoke") { send_keys :delete }

    assert_text "Authorization revoked."
    assert_no_selector "#oauth_grants", text: "markupp-cli"
  end
end
