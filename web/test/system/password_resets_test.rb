require "application_system_test_case"

class PasswordResetsTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "a forgotten password is reset with the emailed token and the new one logs in" do
    api = use_fake_api

    visit new_session_path
    click_on "Forgot password?"
    fill_in "Email", with: EMAIL
    click_on "Send"
    assert_text "If an account uses that email, a reset token is on its way."
    click_on "I have a token"
    fill_in "Token", with: api.sent_tokens.last.fetch(:token)
    fill_in "New password", with: "senha-nova"
    click_on "Reset password"

    assert_text "Password changed. Log in with the new one."
    sign_in_through_form(EMAIL, password: "senha-nova")
  end
end
