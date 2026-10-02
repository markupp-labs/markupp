require "test_helper"

class NotesControllerTest < ActionDispatch::IntegrationTest
  test "index shows the note tree in the sidebar" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" }, { id: "2", path: "inbox.md", content: "" } ])

    get root_path

    assert_response :success
    assert_select "nav[aria-label=?] details summary", "Notes", text: "work"
    assert_select "nav li", text: "plan.md"
    assert_select "nav li", text: "inbox.md"
  end

  test "index shows an empty state when there are no notes" do
    use_fake_api(notes: [])

    get root_path

    assert_response :success
    assert_select "nav", text: /No notes yet/
  end

  test "index links each note of the tree to its page" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])

    get root_path

    assert_select "nav a[href=?]", note_path("1"), text: "plan.md"
    assert_select "nav details[open]", count: 0
  end

  test "show puts the exact markdown in the editor textarea" do
    content = "\n# Plan  \n\n\n- keep   \n\ttabs & <tags>\n\n"
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: } ])

    get note_path("1")

    assert_response :success
    assert_equal content, css_select("main textarea").sole.text
    assert_select "main", text: /work\/plan\.md/
  end

  test "show marks the current note and opens only its ancestors" do
    use_fake_api(notes: [ { id: "1", path: "work/2026/plan.md", content: "" }, { id: "2", path: "work/ideas.md", content: "" },
                          { id: "3", path: "home/list.md", content: "" } ])

    get note_path("1")

    assert_select "nav a[aria-current=page]", text: "plan.md", count: 1
    assert_select "nav a[aria-current]", count: 1
    assert_select "nav details[open] > summary", text: "work"
    assert_select "nav details[open] details[open] > summary", text: "2026"
    assert_select "nav details:not([open]) > summary", text: "home"
  end

  test "show answers not found for an unknown note" do
    use_fake_api(notes: [])

    get note_path("missing")

    assert_response :not_found
    assert_select "main", text: /Note not found/
  end

  test "index answers bad gateway when the API is unreachable" do
    use_fake_api(unreachable: true)

    get root_path

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end
end
