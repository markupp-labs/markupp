require "test_helper"

# Pedido do email de verificação: a API manda o token para o email da conta logada.
class EmailVerificationsControllerTest < ActionDispatch::IntegrationTest
  UNVERIFIED = { email: "caio@markupp.dev", password: "senha-certa", verified: false }.freeze

  test "create asks for login first" do
    post email_verification_path

    assert_redirected_to new_session_path
  end

  test "create sends the confirmation email and says so" do
    api = use_fake_api(accounts: [ UNVERIFIED ], tenants: [])
    sign_in_as(UNVERIFIED[:email])

    post email_verification_path

    assert_redirected_to settings_account_path
    assert_equal "Confirmation email sent. It may take a few minutes.", flash[:notice]
    assert_equal [ [ :email_verification, UNVERIFIED[:email] ] ], api.sent_tokens.map { it.values_at(:kind, :email) }
  end

  test "create for a confirmed email says it is confirmed, not an error" do
    use_fake_api
    sign_in_as

    post email_verification_path

    assert_redirected_to settings_account_path
    assert_equal "Your email is already confirmed.", flash[:notice]
  end

  test "create says when the server can't send email" do
    use_fake_api(accounts: [ UNVERIFIED ], tenants: [], email_enabled: false)
    sign_in_as(UNVERIFIED[:email])

    post email_verification_path

    assert_redirected_to settings_account_path
    assert_equal "This server can't send email. Ask an administrator to confirm your email.", flash[:alert]
  end

  test "create answers bad gateway when the API is unreachable" do
    api = use_fake_api(accounts: [ UNVERIFIED ], tenants: [])
    sign_in_as(UNVERIFIED[:email])
    api.cut_connection!

    post email_verification_path

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  test "create stops after five requests in a minute" do
    api = use_fake_api(accounts: [ UNVERIFIED ], tenants: [])
    sign_in_as(UNVERIFIED[:email])
    6.times { post email_verification_path }

    assert_redirected_to settings_account_path
    assert_equal "Too many attempts. Try again in 1 minute.", flash[:alert]
    assert_equal 5, api.sent_tokens.size
  end
end
