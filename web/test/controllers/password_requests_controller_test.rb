require "test_helper"

# Esqueci a senha: o pedido manda o token por email, e a resposta não revela se a conta existe.
class PasswordRequestsControllerTest < ActionDispatch::IntegrationTest
  SENT = "If an account uses that email, a reset token is on its way."

  test "new shows the email form in the public layout with a way to the token" do
    get new_password_request_path

    assert_response :success
    assert_select "h1", "Reset your password"
    assert_select "form[action=?][method=post]", password_request_path do
      assert_select "label[for=password_request_email]", "Email"
      assert_select "input[type=email][name=?][required]", "password_request[email]"
      assert_select "button", "Send"
    end
    assert_select "a[href=?]", new_password_reset_path
    assert_select "a[href=?]", new_session_path
    assert_select "nav#sidebar", count: 0
  end

  test "the login page points to the password reset" do
    get new_session_path

    assert_select "a[href=?]", new_password_request_path, text: "Forgot password?"
  end

  test "create sends the token by email and says so" do
    api = use_fake_api

    request_reset(FakeMarkuppAccounts::DEFAULT_EMAIL)

    assert_redirected_to new_password_request_path
    assert_equal SENT, flash[:notice]
    assert_equal [ FakeMarkuppAccounts::DEFAULT_EMAIL ], api.sent_tokens.map { it[:email] }
  end

  test "create says the same for an email without account" do
    api = use_fake_api

    request_reset("ninguem@markupp.dev")

    assert_redirected_to new_password_request_path
    assert_equal SENT, flash[:notice]
    assert_empty api.sent_tokens
  end

  test "create with a blank email asks for it under the field" do
    use_fake_api

    request_reset("")

    assert_response :unprocessable_content
    assert_select "input[name=?][aria-invalid=true][aria-describedby=password_request_email_error]", "password_request[email]"
    assert_select "#password_request_email_error", "Enter the email of your account."
  end

  test "create says when the server can't send email" do
    use_fake_api(email_enabled: false)

    request_reset(FakeMarkuppAccounts::DEFAULT_EMAIL)

    assert_response :unprocessable_content
    assert_select "[role=alert]", "This server can't send email. Ask an administrator to reset your password."
    assert_select "input[name=?][value=?]", "password_request[email]", FakeMarkuppAccounts::DEFAULT_EMAIL
  end

  test "create says when the server has password login disabled" do
    use_fake_api.disable_local_auth!

    request_reset(FakeMarkuppAccounts::DEFAULT_EMAIL)

    assert_response :unprocessable_content
    assert_select "[role=alert]", "Password login is disabled on this server."
  end

  test "create answers bad gateway when the API is unreachable" do
    use_fake_api(unreachable: true)

    request_reset(FakeMarkuppAccounts::DEFAULT_EMAIL)

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  test "create stops after five requests in a minute" do
    api = use_fake_api
    6.times { request_reset(FakeMarkuppAccounts::DEFAULT_EMAIL) }

    assert_redirected_to new_password_request_path
    assert_equal "Too many attempts. Try again in 1 minute.", flash[:alert]
    assert_equal 5, api.sent_tokens.size
  end

  private
    def request_reset(email)
      post password_request_path, params: { password_request: { email: } }
    end
end
