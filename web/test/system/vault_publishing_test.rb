require "application_system_test_case"

# O owner publica o cofre nas configurações, copia o link público, abre a leitura sem conta e despublica.
class VaultPublishingTest < ApplicationSystemTestCase
  NOTES = [ { id: "plan", path: "plan.md", content: "# Plan\n" } ].freeze

  test "publish, copy the public link and unpublish the vault" do
    use_fake_api(notes: NOTES)
    sign_in_through_form
    visit settings_publication_path

    fill_in "Slug", with: "acme-notes"
    fill_in "Title", with: "Acme notes"
    click_on "Publish"

    link = "#{Capybara.current_session.server.base_url}/p/acme-notes"
    assert_text "Vault published at #{link}."
    grant_clipboard
    click_on "Copy link"
    assert_selector "[data-controller=copy-link] [role=status]", text: "Copied"
    assert_equal link, evaluate_async_script("navigator.clipboard.readText().then(arguments[0])")

    accept_panel_confirm("Unpublish acme-notes?", button: "Unpublish") { click_on "Unpublish" }

    assert_text "Vault unpublished."
    assert_button "Publish"
  end
end
