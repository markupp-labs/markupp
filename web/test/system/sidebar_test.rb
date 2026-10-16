require "application_system_test_case"

class SidebarTest < ApplicationSystemTestCase
  test "the footer stays in view while a long note list scrolls" do
    use_fake_api(notes: (1..80).map { { id: it.to_s, path: "note-#{it}.md", content: "" } })
    sign_in_through_form

    visit root_path

    assert find("nav > .sidebar-scroll").evaluate_script("this.scrollHeight > this.clientHeight")
    footer_bottom = evaluate_script("document.querySelector('nav > footer').getBoundingClientRect().bottom")
    assert_operator footer_bottom, :<=, evaluate_script("window.innerHeight")
  end

  test "the tenant name leads home from a note" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    within("#current_tenant") { click_on "Acme research" }

    assert_current_path root_path
  end
end
