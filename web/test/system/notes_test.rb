require "application_system_test_case"

class NotesTest < ApplicationSystemTestCase
  test "opening a note mounts the editor with its markdown" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n\nShip *it* with [[ideas]]." } ])

    visit root_path
    find("summary", text: "work").click
    click_on "plan.md"

    assert_selector ".cm-editor .cm-content", text: "Ship it with [[ideas]]."
    assert_selector ".cm-wikilink", text: "[[ideas]]"
    assert_no_selector "textarea"
  end
end
