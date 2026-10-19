require "test_helper"

# Confirmação de email pelo token: só a conta logada que pediu confirma, e depois vê os convites dela.
class EmailConfirmationsControllerTest < ActionDispatch::IntegrationTest
  UNVERIFIED = { email: "caio@markupp.dev", password: "senha-certa", verified: false }.freeze
  INVITE = { id: "invite-1", tenant_id: "tenant-1", email: UNVERIFIED[:email], role: "member" }.freeze

  test "new asks for login first" do
    get new_email_confirmation_path

    assert_redirected_to new_session_path
  end

  test "new shows the token form" do
    use_fake_api
    sign_in_as

    get new_email_confirmation_path

    assert_response :success
    assert_select "h1", "Confirm your email"
    assert_select "main form[action=?][method=post]", email_confirmation_path do
      assert_select "label[for=email_confirmation_token]", "Token"
      assert_select "input[type=text][name=?][required]", "email_confirmation[token]"
      assert_select "input[type=password]", count: 0
      assert_select "button", "Confirm"
    end
    assert_select "main nav a[href=?][aria-current=page]", settings_account_path, text: "Account"
  end

  test "new fills the token from the link" do
    use_fake_api
    sign_in_as

    get new_email_confirmation_path(token: "tok-1")

    assert_select "input[name=?][value=tok-1]", "email_confirmation[token]"
  end

  test "create confirms the email and the pending invites show up" do
    api = use_fake_api(accounts: [ UNVERIFIED ], tenants: [ FakeMarkuppTenants::DEFAULT_TENANTS.first ], invites: [ INVITE ])
    sign_in_as(UNVERIFIED[:email])
    post email_verification_path

    confirm_email(api.sent_tokens.last.fetch(:token))

    assert_redirected_to settings_account_path
    assert_equal "Email confirmed. Pending invites now show up in your tenant list.", flash[:notice]
    get tenants_path
    assert_select "main li", text: /Join Acme research as member/
  end

  test "create with a token the API refuses says so under the field" do
    use_fake_api(accounts: [ UNVERIFIED ], tenants: [])
    sign_in_as(UNVERIFIED[:email])

    confirm_email("tok-9")

    assert_response :unprocessable_content
    assert_select "input[name=?][value=tok-9][aria-invalid=true][aria-describedby=email_confirmation_token_error]", "email_confirmation[token]"
    assert_select "#email_confirmation_token_error", "This token is invalid, expired or already used."
  end

  test "create for a confirmed email says it is confirmed, not an error" do
    use_fake_api
    sign_in_as

    confirm_email("tok-1")

    assert_redirected_to settings_account_path
    assert_equal "Your email is already confirmed.", flash[:notice]
  end

  test "create answers bad gateway when the API is unreachable" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    confirm_email("tok-1")

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  private
    def confirm_email(token)
      post email_confirmation_path, params: { email_confirmation: { token: } }
    end
end
