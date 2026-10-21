require "application_system_test_case"

# Consentimento OAuth no navegador: o cliente de volta é o próprio painel (/up), para o teste ver onde o navegador para.
class OAuthAuthorizationsTest < ApplicationSystemTestCase
  REQUEST = { id: "r1", client_name: "Claude", scopes: %w[notas:leitura] }.freeze

  setup { @callback = "#{Capybara.current_session.server.base_url}/up" }

  test "approving with Enter goes back to the client with the code" do
    use_fake_api(oauth_requests: [ consent_request ])
    sign_in_through_form

    visit "/oauth/authorize/r1"
    assert_text "Claude wants to access your markupp"
    assert_equal "Approve", evaluate_script("document.activeElement.textContent.trim()")
    page.driver.browser.action.send_keys(:enter).perform

    assert_current_path "/up?code=code-r1&state=state-r1"
  end

  test "denying with Esc goes back to the client with the refusal" do
    use_fake_api(oauth_requests: [ consent_request ])
    sign_in_through_form

    visit "/oauth/authorize/r1"
    assert_text "Claude wants to access your markupp"
    page.driver.browser.action.send_keys(:escape).perform

    assert_current_path "/up?error=access_denied&state=state-r1"
  end

  test "denying with the button goes back to the client with the refusal" do
    use_fake_api(oauth_requests: [ consent_request ])
    sign_in_through_form

    visit "/oauth/authorize/r1"
    assert_button "Deny"
    click_on "Deny"

    assert_current_path "/up?error=access_denied&state=state-r1"
  end

  test "a client already authorized goes straight back with the code" do
    use_fake_api(oauth_requests: [ consent_request(already_authorized: true) ])
    sign_in_through_form

    visit "/oauth/authorize/r1"

    assert_current_path "/up?code=code-r1&state=state-r1"
  end

  test "without a session the login comes first and the consent after it" do
    use_fake_api(oauth_requests: [ consent_request ])

    visit "/oauth/authorize/r1"
    assert_current_path new_session_path
    fill_in "Email", with: FakeMarkuppAccounts::DEFAULT_EMAIL
    fill_in "Password", with: FakeMarkuppAccounts::DEFAULT_PASSWORD
    click_on "Log in"

    assert_text "Claude wants to access your markupp"
    click_on "Approve"
    assert_current_path "/up?code=code-r1&state=state-r1"
  end

  test "an expired request shows the error page" do
    use_fake_api(oauth_requests: [ consent_request(expires_at: 1.minute.ago.utc.iso8601) ])
    sign_in_through_form

    visit "/oauth/authorize/r1"

    assert_text "This authorization request is no longer valid"
    assert_no_button "Approve"
  end

  private
    def consent_request(**overrides) = REQUEST.merge(redirect_uri: @callback, **overrides)
end
