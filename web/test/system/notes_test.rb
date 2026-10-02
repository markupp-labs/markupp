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

  test "ctrl+s saves the edited note" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])

    visit note_path("1")
    find(".cm-content").send_keys([ :control, :end ], "Ship it.")
    assert_text "Unsaved changes"
    find(".cm-content").send_keys([ :control, "s" ])

    assert_text "Saved"
    assert_no_text "Unsaved changes"
    assert_equal "# Plan\nShip it.", api.put_requests.sole[:content]
  end

  test "ctrl+s creates a new note in the folder of the open note" do
    api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])

    visit note_path("1")
    click_on "New note"
    assert_text "work/Untitled.md"
    find(".cm-content").send_keys("Fresh idea.")
    find(".cm-content").send_keys([ :control, "s" ])

    assert_text "Created"
    assert_selector "nav a[aria-current=page]", text: "Untitled.md"
    assert_equal({ path: "work/Untitled.md", content: "Fresh idea." }, api.post_requests.sole)
  end
end
