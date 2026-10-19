require "test_helper"

# Redefinir a senha com o token do email: público, a senha nova vale no próximo login.
class PasswordResetsControllerTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "new shows the token form in the public layout" do
    get new_password_reset_path

    assert_response :success
    assert_select "h1", "Choose a new password"
    assert_select "form[action=?][method=post]", password_reset_path do
      assert_select "label[for=password_reset_token]", "Token"
      assert_select "input[type=text][name=?][required]", "password_reset[token]"
      assert_select "label[for=password_reset_password]", "New password"
      assert_select "input[type=password][name=?][required][autocomplete=new-password]", "password_reset[password]"
      assert_select "button", "Reset password"
    end
    assert_select "nav#sidebar", count: 0
  end

  test "new leads back to the login before the title" do
    get new_password_reset_path

    assert_select "main a[href=?] ~ h1", new_session_path
    assert_select "main a[href=?]", new_session_path, text: "Back to log in" do
      assert_select "svg[data-icon=arrow-left]"
    end
  end

  test "new fills the token from the link" do
    get new_password_reset_path(token: "tok-1")

    assert_select "input[name=?][value=tok-1]", "password_reset[token]"
  end

  test "create changes the password and sends to the login, where the new one works" do
    token = emailed_token(use_fake_api)

    reset_password(token, "senha-nova")

    assert_redirected_to new_session_path
    assert_equal "Password changed. Log in with the new one.", flash[:notice]
    sign_in_as(EMAIL, password: "senha-nova")
    assert_redirected_to root_path
  end

  test "create with a token the API refuses says so under the field and keeps the token" do
    use_fake_api

    reset_password("tok-9", "senha-nova")

    assert_response :unprocessable_content
    assert_select "input[name=?][value=tok-9][aria-invalid=true][aria-describedby=password_reset_token_error]", "password_reset[token]"
    assert_select "#password_reset_token_error", "This token is invalid, expired or already used."
    assert_select "input[name=?]:not([value])", "password_reset[password]"
  end

  test "a used token does not work twice" do
    token = emailed_token(use_fake_api)
    reset_password(token, "senha-nova")

    reset_password(token, "outra-senha")

    assert_response :unprocessable_content
    assert_select "#password_reset_token_error", "This token is invalid, expired or already used."
  end

  test "create with a password the API refuses says the rule under the field" do
    token = emailed_token(use_fake_api)

    reset_password(token, " ")

    assert_response :unprocessable_content
    assert_select "#password_reset_password_error", "Enter a password up to 72 bytes long."
  end

  test "create says when the server can't send email" do
    use_fake_api(email_enabled: false)

    reset_password("tok-1", "senha-nova")

    assert_response :unprocessable_content
    assert_select "[role=alert]", "This server can't send email, so password reset is off. Ask an administrator."
  end

  test "create answers bad gateway when the API is unreachable" do
    use_fake_api(unreachable: true)

    reset_password("tok-1", "senha-nova")

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  private
    def emailed_token(api)
      post password_request_path, params: { password_request: { email: EMAIL } }
      api.sent_tokens.last.fetch(:token)
    end

    def reset_password(token, password)
      post password_reset_path, params: { password_reset: { token:, password: } }
    end
end
