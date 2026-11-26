require "application_system_test_case"

class PasswordResetsTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "a forgotten password is reset with the emailed token and the new one logs in" do
    api = use_fake_api

    visit new_session_path
    click_on "Forgot password?"
    fill_in "Email", with: EMAIL
    click_on "Send"
    assert_text "Check your email for a link to reset your password. If it doesn't show up in a few minutes, check your spam folder."
    click_on "I have a code"
    fill_in "Code", with: api.sent_tokens.last.fetch(:token)
    fill_in "New password", with: "senha-nova-de-teste"
    fill_in "Confirm password", with: "senha-nova-de-teste"
    click_on "Reset password"

    assert_text "Password changed. Log in with the new one."
    sign_in_through_form(EMAIL, password: "senha-nova-de-teste")
  end
end
