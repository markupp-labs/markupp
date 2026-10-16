require "test_helper"

# Página da conta: email e confirmação dele, também para quem ainda não tem tenant.
class AccountsControllerTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "show asks for login first" do
    get account_path

    assert_redirected_to new_session_path
  end

  test "show has the email and the ways to confirm it" do
    use_fake_api
    sign_in_as

    get account_path

    assert_response :success
    assert_select "h1", "Account"
    assert_select "main", text: /#{EMAIL}/
    assert_select "main h2", "Email confirmation"
    assert_select "main form[action=?][method=post] button", email_verification_path, text: "Send confirmation email"
    assert_select "main a[href=?]", new_email_confirmation_path, text: "I have a token"
    assert_select "nav a[href=?]", root_path, text: "Back to notes"
  end

  test "the sidebar footer links the email to the account" do
    use_fake_api
    sign_in_as

    get account_path

    assert_select "nav footer a[href=?][aria-current=page]", account_path, text: EMAIL
  end

  test "show works for an account without tenant" do
    use_fake_api(tenants: [])
    sign_in_as

    get account_path

    assert_response :success
    assert_select "nav footer a[href=?]", account_path
  end

  test "the empty invite list of the onboarding points to the account" do
    use_fake_api(tenants: [])
    sign_in_as

    get tenants_path

    assert_select "main a[href=?]", account_path, text: "Confirm your email"
  end
end
