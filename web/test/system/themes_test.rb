require "application_system_test_case"

# O Chromium headless segue o tema claro do sistema, então o botão visível começa sendo o escuro.
class ThemesTest < ApplicationSystemTestCase
  test "switching the theme repaints the page right away" do
    use_fake_api(notes: [])

    visit root_path
    click_on "Dark theme"
    assert_selector "html[data-theme=dark]", visible: :all

    click_on "Light theme"
    assert_selector "html[data-theme=light]", visible: :all
  end
end
