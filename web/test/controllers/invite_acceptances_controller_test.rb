require "test_helper"

# Aceite público por token: quem ainda não tem conta cria a sua com o token do email do convite.
class InviteAcceptancesControllerTest < ActionDispatch::IntegrationTest
  NEW_EMAIL = "caio@markupp.dev"
  INVITE = { id: "invite-1", tenant_id: "tenant-1", email: NEW_EMAIL, role: "member", token: "tok-1" }.freeze

  test "new shows the token form in the public layout" do
    get new_invite_acceptance_path

    assert_response :success
    assert_select "h1", "Accept an invite"
    assert_select "main", text: /Paste the token from the invite email and choose a password for your new account\./
    assert_select "form[action=?][method=post]", invite_acceptance_path do
      assert_select "label[for=invite_acceptance_token]", "Token"
      assert_select "input[type=text][name=?][required]", "invite_acceptance[token]"
      assert_select "label[for=invite_acceptance_password]", "Password"
      assert_select "input[type=password][name=?][required][autocomplete=new-password]", "invite_acceptance[password]"
      assert_select "button", "Create account"
    end
    assert_select "nav#sidebar", count: 0
  end

  test "new leads back to the login before the title" do
    get new_invite_acceptance_path

    assert_select "main a[href=?] ~ h1", new_session_path
    assert_select "main a[href=?]", new_session_path, text: "Back to log in" do
      assert_select "svg[data-icon=arrow-left]"
    end
  end

  test "new fills the token from the link" do
    get new_invite_acceptance_path(token: "tok-1")

    assert_select "input[name=?][value=tok-1]", "invite_acceptance[token]"
  end

  test "the login page points to the invite acceptance" do
    get new_session_path

    assert_select "main", text: /Got an invite token\?/
    assert_select "a[href=?]", new_invite_acceptance_path, text: "Accept it here."
  end

  test "create makes the account and opens the notes of the tenant of the invite" do
    api = use_fake_api(invites: [ INVITE ])

    accept_invite("tok-1", "senha-nova")

    assert_redirected_to root_path
    follow_redirect!
    assert_response :success
    assert_select "#account_menu", text: /#{NEW_EMAIL}/
    assert_select "nav #current_tenant", text: /Acme research.*member/m
    assert_equal "tenant-1", api.received_credentials.reverse.find { it[:call] == "GET /notes" }[:tenant_id]
  end

  test "create with a token the API refuses says so under the field and keeps the token" do
    use_fake_api(invites: [ INVITE ])

    accept_invite("tok-9", "senha-nova")

    assert_response :unprocessable_content
    assert_select "input[name=?][value=tok-9][aria-invalid=true][aria-describedby=invite_acceptance_token_error]", "invite_acceptance[token]"
    assert_select "#invite_acceptance_token_error", "This token is invalid, expired or already used."
    assert_select "input[name=?]:not([value])", "invite_acceptance[password]"
  end

  test "create with a blank token asks for it" do
    use_fake_api(invites: [ INVITE ])

    accept_invite("", "senha-nova")

    assert_response :unprocessable_content
    assert_select "#invite_acceptance_token_error", "Paste the token from the email."
  end

  test "create with a password the API refuses says the rule under the field" do
    use_fake_api(invites: [ INVITE ])

    accept_invite("tok-1", " ")

    assert_response :unprocessable_content
    assert_select "input[name=?][aria-invalid=true][aria-describedby=invite_acceptance_password_error]", "invite_acceptance[password]"
    assert_select "#invite_acceptance_password_error", "Enter a password up to 72 bytes long."
  end

  test "create for an email that already has an account points to the login" do
    use_fake_api(invites: [ INVITE.merge(email: FakeMarkuppAccounts::DEFAULT_EMAIL) ])

    accept_invite("tok-1", "senha-nova")

    assert_response :unprocessable_content
    assert_select "[role=alert]", text: /This email already has an account\. Log in and accept the invite from your tenant list\./ do
      assert_select "a[href=?]", new_session_path, text: "Log in"
    end
  end

  test "create says when the server has password login disabled" do
    use_fake_api(invites: [ INVITE ]).disable_local_auth!

    accept_invite("tok-1", "senha-nova")

    assert_response :unprocessable_content
    assert_select "[role=alert]", "Password login is disabled on this server."
  end

  test "a used token does not work twice" do
    use_fake_api(invites: [ INVITE ])
    accept_invite("tok-1", "senha-nova")
    delete session_path

    accept_invite("tok-1", "outra-senha")

    assert_response :unprocessable_content
    assert_select "#invite_acceptance_token_error", "This token is invalid, expired or already used."
  end

  test "create answers bad gateway when the API is unreachable" do
    use_fake_api(unreachable: true)

    accept_invite("tok-1", "senha-nova")

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  private
    def accept_invite(token, password)
      post invite_acceptance_path, params: { invite_acceptance: { token:, password: } }
    end
end
