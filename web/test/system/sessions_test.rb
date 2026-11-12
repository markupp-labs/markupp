require "application_system_test_case"

class SessionsTest < ApplicationSystemTestCase
  test "logging in opens the page asked and logging out goes back to the login" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])

    visit note_path("1")
    assert_current_path new_session_path
    fill_in "Email", with: FakeMarkuppAccounts::DEFAULT_EMAIL
    fill_in "Password", with: FakeMarkuppAccounts::DEFAULT_PASSWORD
    click_on "Log in"

    assert_field "Path", with: "/plan"
    click_on FakeMarkuppAccounts::DEFAULT_EMAIL
    click_on "Log out"
    assert_text "Logged out."
    assert_current_path new_session_path
  end

  test "the eye shows the typed password and hides it again" do
    visit new_session_path
    fill_in "Password", with: "senha-de-teste-longa"

    click_on "Show password"
    assert_field "Password", type: "text", with: "senha-de-teste-longa"
    assert_selector "button[aria-pressed=true][aria-label='Show password']"

    click_on "Show password"
    assert_field "Password", type: "password"
  end
end
