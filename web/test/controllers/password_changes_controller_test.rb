require "test_helper"

# Troca de senha pela Conta: o painel pede o email de redefinição para a própria conta logada.
class PasswordChangesControllerTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "create asks for login first" do
    post password_change_path

    assert_redirected_to new_session_path
  end

  test "create sends the reset email to the account and says so" do
    api = use_fake_api
    sign_in_as

    post password_change_path

    assert_redirected_to settings_account_path
    assert_equal "Password reset email sent to #{EMAIL}. Use its token to choose the new password.", flash[:notice]
    assert_equal [ [ :password_reset, EMAIL ] ], api.sent_tokens.map { it.values_at(:kind, :email) }
  end

  test "create says when the server can't send email" do
    use_fake_api(email_enabled: false)
    sign_in_as

    post password_change_path

    assert_redirected_to settings_account_path
    assert_equal "This server can't send email. Ask an administrator to reset your password.", flash[:alert]
  end

  test "create says when password login is disabled" do
    api = use_fake_api
    sign_in_as
    api.disable_local_auth!

    post password_change_path

    assert_redirected_to settings_account_path
    assert_equal "Password login is disabled on this server.", flash[:alert]
  end

  test "create answers bad gateway when the API is unreachable" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    post password_change_path

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  test "create stops after five requests in a minute" do
    api = use_fake_api
    sign_in_as
    6.times { post password_change_path }

    assert_redirected_to settings_account_path
    assert_equal "Too many attempts. Try again in 1 minute.", flash[:alert]
    assert_equal 5, api.sent_tokens.size
  end
end
