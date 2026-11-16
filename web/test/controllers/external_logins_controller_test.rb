require "test_helper"

# Login, cadastro e vínculo por provedor externo (ADR-0054). O painel guarda state, verifier e nonce na sessão, confere
# o state na volta e repassa o resto ao servidor, que troca o código com o segredo do provedor.
class ExternalLoginsControllerTest < ActionDispatch::IntegrationTest
  GITHUB = FakeMarkuppExternalLogin::DEFAULT_PROVIDERS

  # Começa a tentativa e devolve o state que o provedor devolveria na volta.
  def begin_attempt(purpose)
    post external_login_path("github"), params: { purpose: }
    assert_response :redirect
    Rack::Utils.parse_query(URI(response.location).query).fetch("state")
  end

  def come_back(code, state)
    get external_login_callback_path("github"), params: { code:, state: }
  end

  test "the login page offers each provider of the server" do
    use_fake_api(login_providers: GITHUB)

    get new_session_path

    assert_select "form[action=?][method=post]", external_login_path("github") do
      assert_select "input[type=hidden][name=purpose][value=login]"
      assert_select "button", "Continue with GitHub"
    end
  end

  test "the account creation page offers each provider to sign up" do
    use_fake_api(login_providers: GITHUB)

    get new_registration_path

    assert_select "form[action=?] input[name=purpose][value=signup]", external_login_path("github")
  end

  test "a server without providers shows no provider button" do
    use_fake_api

    get new_session_path

    assert_select "form[action^='/auth/']", count: 0
  end

  test "create sends the browser to the provider" do
    use_fake_api(login_providers: GITHUB)

    post external_login_path("github"), params: { purpose: "login" }

    assert_redirected_to "https://github.example/authorize?state=state-1"
  end

  test "a provider that does not answer keeps the person on the login page" do
    use_fake_api(login_providers: GITHUB).take_provider_down!

    post external_login_path("github"), params: { purpose: "login" }

    assert_redirected_to new_session_path
    assert_equal "GitHub did not answer. Try again later.", flash[:alert]
  end

  test "login with a linked identity signs in" do
    use_fake_api(login_providers: GITHUB).link_identity!("codigo-ana", FakeMarkuppAccounts::DEFAULT_EMAIL)

    come_back("codigo-ana", begin_attempt("login"))

    assert_redirected_to root_path
    follow_redirect!
    follow_redirect! while response.redirect?
    assert_select "#account_menu", text: /#{FakeMarkuppAccounts::DEFAULT_EMAIL}/
  end

  test "login with an unlinked identity creates no account and points to the sign up" do
    use_fake_api(login_providers: GITHUB)

    come_back("codigo-novo", begin_attempt("login"))

    assert_redirected_to new_registration_path
    assert_equal "No account uses this GitHub identity yet. Create an account with it, or log in and connect it in Settings.", flash[:alert]
    get root_path
    assert_redirected_to new_session_path
  end

  test "a callback with another state is refused without reaching the server" do
    use_fake_api(login_providers: GITHUB).link_identity!("codigo-ana", FakeMarkuppAccounts::DEFAULT_EMAIL)
    begin_attempt("login")

    come_back("codigo-ana", "state-de-outra-aba")

    assert_redirected_to new_session_path
    assert_equal "This sign in expired or came from another tab. Start again.", flash[:alert]
  end

  test "a callback without an attempt is refused" do
    use_fake_api(login_providers: GITHUB)

    come_back("codigo-ana", "state-1")

    assert_redirected_to new_session_path
    assert_equal "This sign in expired or came from another tab. Start again.", flash[:alert]
  end

  test "the attempt is used once" do
    use_fake_api(login_providers: GITHUB).link_identity!("codigo-ana", FakeMarkuppAccounts::DEFAULT_EMAIL)
    state = begin_attempt("login")
    come_back("codigo-ana", state)
    delete session_path

    come_back("codigo-ana", state)

    assert_redirected_to new_session_path
  end

  test "a person who cancels at the provider goes back to the login" do
    use_fake_api(login_providers: GITHUB)
    state = begin_attempt("login")

    get external_login_callback_path("github"), params: { error: "access_denied", state: }

    assert_redirected_to new_session_path
    assert_equal "Sign in with GitHub was cancelled.", flash[:alert]
  end

  test "a code the provider refuses says to start again" do
    use_fake_api(login_providers: GITHUB)

    come_back("codigo-falso", begin_attempt("login"))

    assert_redirected_to new_session_path
    assert_equal "GitHub refused this sign in. Start again.", flash[:alert]
  end

  test "sign up creates the account with the provider email and signs in" do
    use_fake_api(login_providers: GITHUB)

    come_back("codigo-novo", begin_attempt("signup"))

    assert_redirected_to root_path
    follow_redirect!
    follow_redirect! while response.redirect?
    assert_select "#account_menu", text: /novo@markupp.dev/
  end

  test "sign up with an email that has an account says to log in and connect" do
    use_fake_api(login_providers: GITHUB)

    come_back("codigo-ana", begin_attempt("signup"))

    assert_redirected_to new_session_path
    assert_equal "This email already has an account. Log in and connect GitHub in Settings > Account.", flash[:alert]
  end

  test "sign up on a server that only creates accounts by invite says so" do
    use_fake_api(login_providers: GITHUB).close_registration!

    come_back("codigo-novo", begin_attempt("signup"))

    assert_redirected_to new_registration_path
    assert_equal "This server only creates accounts by invite. Ask an admin for one.", flash[:alert]
  end

  test "connecting needs a session" do
    use_fake_api(login_providers: GITHUB)

    post external_login_path("github"), params: { purpose: "link" }

    assert_redirected_to new_session_path
  end

  test "an unknown purpose is refused" do
    use_fake_api(login_providers: GITHUB)

    post external_login_path("github"), params: { purpose: "admin" }

    assert_redirected_to new_session_path
  end
end
