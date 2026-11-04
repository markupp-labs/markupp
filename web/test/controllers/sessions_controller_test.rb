require "test_helper"

class SessionsControllerTest < ActionDispatch::IntegrationTest
  test "new shows the login form in the public layout" do
    get new_session_path

    assert_response :success
    assert_select "h1", "Log in"
    assert_select "form[action=?]", session_path do
      assert_select "label", "Email"
      assert_select "input[type=email][name=?][required]", "session[email]"
      assert_select "label", "Password"
      assert_select "input[type=password][name=?][required]", "session[password]"
      assert_select "button", "Log in"
    end
    assert_select "nav#sidebar", count: 0
    assert_select "form.theme-to-dark button[aria-label=?]", "Dark theme"
    assert_select "a[href=?]", new_session_path, count: 0
  end

  test "pages ask for login first" do
    get root_path

    assert_redirected_to new_session_path
  end

  test "create logs in and opens the notes with the email in the sidebar" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])

    sign_in_as

    assert_redirected_to root_path
    follow_redirect!
    assert_response :success
    assert_select "nav [role=treeitem]", text: "└── plan"
    assert_select "#account_menu", text: /#{FakeMarkuppAccounts::DEFAULT_EMAIL}/
    assert_select "#account_menu form[action=?] button", session_path, text: "Log out"
  end

  test "create goes back to the page asked before login" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    get note_path("1")

    sign_in_as

    assert_redirected_to note_path("1")
  end

  # //notes seria outro site para o navegador; a volta fica no caminho do painel.
  test "create goes back to a path of the panel even when asked with a double slash" do
    use_fake_api
    get "/", env: { "PATH_INFO" => "//notes" }
    assert_redirected_to new_session_path

    sign_in_as

    assert_equal "http://www.example.com/notes", response.location
  end

  test "create does not go back to a form submitted before login" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    patch note_path("1"), params: { note: { path: "plan.md", content: "x" } }
    assert_redirected_to new_session_path

    sign_in_as

    assert_redirected_to root_path
  end

  test "create does not go back to a frame asked before login" do
    use_fake_api
    get my_invites_path, headers: { "Turbo-Frame" => "my_invites" }

    assert_select "turbo-frame#my_invites [role=alert] a[href=?][data-turbo-frame=_top]", new_session_path,
                  text: "Your session expired. Log in again."
    sign_in_as

    assert_redirected_to root_path
  end

  test "create with a wrong password keeps the email and says so" do
    use_fake_api

    sign_in_as(FakeMarkuppAccounts::DEFAULT_EMAIL, password: "senha-errada")

    assert_response :unprocessable_content
    assert_select "[role=alert]", "Email or password is wrong."
    assert_select "input[name=?][value=?]", "session[email]", FakeMarkuppAccounts::DEFAULT_EMAIL
    assert_select "input[name=?]:not([value])", "session[password]"
    get root_path
    assert_redirected_to new_session_path
  end

  test "create says when the server has password login disabled" do
    use_fake_api.disable_local_auth!

    sign_in_as

    assert_response :unprocessable_content
    assert_select "[role=alert]", "Password login is disabled on this server."
  end

  test "create answers bad gateway when the API is unreachable" do
    use_fake_api(unreachable: true)

    sign_in_as

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  test "create stops after too many attempts" do
    use_fake_api
    10.times { sign_in_as(FakeMarkuppAccounts::DEFAULT_EMAIL, password: "senha-errada") }

    sign_in_as

    assert_redirected_to new_session_path
    follow_redirect!
    assert_select "[role=alert]", "Too many attempts. Try again in 3 minutes."
  end

  test "the tokens stay out of the page" do
    use_fake_api
    sign_in_as
    follow_redirect!

    assert_no_match(/access-1|refresh-1/, response.body)
  end

  test "notes go to the API with the access token of the session" do
    api = use_fake_api
    sign_in_as

    get root_path

    assert_equal "access-1", api.received_credentials.find { it[:call].start_with?("GET /v1/vaults/") }[:access_token]
  end

  test "an expired access token is renewed without the user noticing" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as
    api.expire_access_tokens!

    get root_path

    assert_response :success
    assert_select "nav [role=treeitem]", text: "└── plan"
  end

  test "a refused renewal ends the session and asks for login again" do
    api = use_fake_api
    sign_in_as
    api.expire_access_tokens!
    api.revoke_refresh_tokens!

    get root_path

    assert_redirected_to new_session_path
    follow_redirect!
    assert_select "[role=alert]", "Your session expired. Log in again."
    get root_path
    assert_redirected_to new_session_path
  end

  test "destroy revokes the session on the API and goes to the login" do
    api = use_fake_api
    sign_in_as

    delete session_path

    assert_redirected_to new_session_path
    assert_not api.refresh_token_valid?("refresh-1")
    follow_redirect!
    assert_select "[role=status]", "Logged out."
    get root_path
    assert_redirected_to new_session_path
  end

  test "destroy logs out even when the API is unreachable" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    delete session_path

    assert_redirected_to new_session_path
    get root_path
    assert_redirected_to new_session_path
  end
end
