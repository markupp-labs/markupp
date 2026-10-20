require "test_helper"

# Consentimento OAuth: o servidor manda o navegador a /oauth/authorize/<id>, a conta logada aprova ou recusa e volta
# ao cliente pelo endereço que a API devolve.
class OAuthAuthorizationsControllerTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  REQUEST = { id: "r1", client_name: "Claude", redirect_host: "claude.ai", scopes: %w[notas:leitura notas:escrita] }.freeze
  APPROVED = "https://claude.ai/api/callback?code=code-r1&state=state-r1".freeze
  DENIED = "https://claude.ai/api/callback?error=access_denied&state=state-r1".freeze

  test "without a session the consent asks for login and comes back after it" do
    use_fake_api(oauth_requests: [ REQUEST ])

    get "/oauth/authorize/r1"
    assert_redirected_to new_session_path
    sign_in_as

    assert_redirected_to "/oauth/authorize/r1"
  end

  test "the consent names the client, where it returns and what it can do" do
    use_fake_api(oauth_requests: [ REQUEST ])
    sign_in_as

    get "/oauth/authorize/r1"

    assert_response :success
    assert_select "h1", "Claude wants to access your markupp"
    assert_select "main", text: /Signed in as #{EMAIL}/
    assert_select "main", text: /Returns to\s*claude\.ai/
    assert_select "main li", text: "Read your notes"
    assert_select "main li", text: "Create, edit and delete your notes"
    assert_select "pre.mascot"
    assert_select "nav#sidebar", count: 0
  end

  test "the consent offers approve with the first focus and deny, both out of Turbo" do
    use_fake_api(oauth_requests: [ REQUEST ])
    sign_in_as

    get "/oauth/authorize/r1"

    assert_select "[data-controller=oauth-consent]" do
      assert_select "form[action=?][data-turbo=false] button.btn[autofocus]", "/oauth/authorize/r1/approve", text: "Approve"
      assert_select "form[action=?][data-turbo=false] button.btn.btn-danger", "/oauth/authorize/r1/deny", text: "Deny"
    end
  end

  test "approving goes back to the client with the address the API answered" do
    api = use_fake_api(oauth_requests: [ REQUEST ])
    sign_in_as

    post "/oauth/authorize/r1/approve"

    assert_redirected_to APPROVED
    assert_response :see_other
    assert_equal [ [ "r1", "approve", EMAIL ] ], api.oauth_decisions
  end

  test "denying goes back to the client with the refusal the API answered" do
    api = use_fake_api(oauth_requests: [ REQUEST ])
    sign_in_as

    post "/oauth/authorize/r1/deny"

    assert_redirected_to DENIED
    assert_response :see_other
    assert_equal [ [ "r1", "deny", EMAIL ] ], api.oauth_decisions
  end

  test "a client already authorized with these scopes is approved without the screen" do
    api = use_fake_api(oauth_requests: [ REQUEST.merge(already_authorized: true) ])
    sign_in_as

    get "/oauth/authorize/r1"

    assert_redirected_to APPROVED
    assert_equal [ [ "r1", "approve", EMAIL ] ], api.oauth_decisions
  end

  test "an expired request shows the error page without a decision" do
    use_fake_api(oauth_requests: [ REQUEST.merge(expires_at: 1.minute.ago.utc.iso8601) ])
    sign_in_as

    get "/oauth/authorize/r1"

    assert_gone_request_page
  end

  test "an unknown request shows the error page" do
    use_fake_api(oauth_requests: [ REQUEST ])
    sign_in_as

    get "/oauth/authorize/r9"

    assert_gone_request_page
  end

  test "deciding a request already decided shows the error page" do
    use_fake_api(oauth_requests: [ REQUEST ])
    sign_in_as
    post "/oauth/authorize/r1/deny"

    post "/oauth/authorize/r1/approve"

    assert_gone_request_page
  end

  test "the consent says when the API is unreachable" do
    api = use_fake_api(oauth_requests: [ REQUEST ])
    sign_in_as
    api.cut_connection!

    get "/oauth/authorize/r1"

    assert_response :bad_gateway
    assert_select "h1", /Can't reach the markupp server/
  end

  private
    def assert_gone_request_page
      assert_response :not_found
      assert_select "h1", /This authorization request is no longer valid/
      assert_select "main", text: /Go back to the app and connect again/
      assert_select "form[action^='/oauth/']", count: 0
    end
end
