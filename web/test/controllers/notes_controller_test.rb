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
    assert_select "main input[name=?][value=?]", "note[path]", "work/plan.md"
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

  test "show puts the note in a save form with the version read" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x", updated_at: "2026-09-30T12:00:00.123Z" } ])

    get note_path("1")

    assert_select "form[action=?]", note_path("1") do
      assert_select "input[type=hidden][name=last_modified_at][value=?]", "2026-09-30T12:00:00.123Z"
      assert_select "input[type=text][name=?][value=?]", "note[path]", "a.md"
      assert_select "button", text: "Save"
    end
  end

  test "update saves the note and redirects with a notice" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "old" } ])

    patch note_path("1"), params: { note: { path: "a.md", content: "new" }, last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_redirected_to note_path("1")
    assert_equal({ path: "a.md", content: "new", lastModifiedAt: "2026-09-30T12:00:00.123Z", force: false }, api.put_requests.sole)
    follow_redirect!
    assert_select "main", text: /Saved/
  end

  test "update on a conflict keeps the text and offers reload or overwrite" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)

    patch note_path("1"), params: { note: { path: "a.md", content: "mine" }, last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_response :conflict
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "input[name=last_modified_at][value=?]", "2026-09-30T12:00:00.123Z"
    assert_select "a[href=?]", note_path("1"), text: "Reload and discard my changes"
    assert_select "button[name=force][value=true]", text: "Overwrite"
  end

  test "update with force overwrites despite the conflict" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)

    patch note_path("1"), params: { note: { path: "a.md", content: "mine" }, last_modified_at: "2026-09-30T12:00:00.123Z", force: "true" }

    assert_redirected_to note_path("1")
    assert api.put_requests.sole[:force]
  end

  test "sidebar links new note to the folder of the open note" do
    use_fake_api(notes: [ { id: "1", path: "work/2026/plan.md", content: "" } ])

    get note_path("1")

    assert_select "nav a[href=?]", new_note_path(folder: "work/2026"), text: "New note"
  end

  test "sidebar links new note to the root when no note is open" do
    use_fake_api(notes: [])

    get root_path

    assert_select "nav a[href=?]", new_note_path, text: "New note"
  end

  test "new opens an empty Untitled.md in the save form" do
    use_fake_api(notes: [])

    get new_note_path(folder: "work")

    assert_response :success
    assert_select "form[action=?][method=post]", notes_path do
      assert_select "input[type=text][name=?][value=?]", "note[path]", "work/Untitled.md"
      assert_select "button", text: "Save"
    end
    assert_select "button", text: "Delete", count: 0
    assert_equal "", css_select("main textarea").sole.text
  end

  test "create posts the note and redirects to it" do
    api = use_fake_api(notes: [])

    post notes_path, params: { note: { path: "work/Untitled.md", content: "# New\n" } }

    assert_redirected_to note_path("created-1")
    assert_equal({ path: "work/Untitled.md", content: "# New\n" }, api.post_requests.sole)
  end

  test "create numbers an untitled note whose name is taken" do
    api = use_fake_api(notes: [ { id: "1", path: "Untitled.md", content: "" } ])

    post notes_path, params: { note: { path: "Untitled.md", content: "" } }

    assert_redirected_to note_path("created-2")
    assert_equal "Untitled 1.md", Note.find(api, "created-2").path
  end

  test "create with a taken path keeps the text and flags the path" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])

    post notes_path, params: { note: { path: "plan.md", content: "mine" } }

    assert_response :conflict
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "input[name=?][value=?][aria-invalid=true]", "note[path]", "plan.md"
    assert_select "#note_path_error", text: "Path already taken"
  end

  test "update renames the note and the tree shows the new name" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])

    patch note_path("1"), params: { note: { path: "b.md", content: "x" } }

    assert_redirected_to note_path("1")
    assert_equal "b.md", api.put_requests.sole[:path]
    follow_redirect!
    assert_select "nav a[aria-current=page]", text: "b.md"
    assert_select "nav li", text: "a.md", count: 0
  end

  test "update moves the note into a folder and opens it in the tree" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])

    patch note_path("1"), params: { note: { path: "work/2026/a.md", content: "x" } }
    follow_redirect!

    assert_select "nav details[open] details[open] a[aria-current=page]", text: "a.md"
    assert_select "input[name=?][value=?]", "note[path]", "work/2026/a.md"
  end

  test "update on a taken path keeps what was typed and shows the error under the field" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "y" } ])

    patch note_path("1"), params: { note: { path: "b.md", content: "mine" }, last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_response :conflict
    assert_select "input[name=?][value=?][aria-invalid=true]", "note[path]", "b.md"
    assert_select "#note_path_error", text: "Path already taken"
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "input[name=last_modified_at][value=?]", "2026-09-30T12:00:00.123Z"
  end

  test "update on a path the API refuses shows its error under the field" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])

    patch note_path("1"), params: { note: { path: "", content: "mine" } }

    assert_response :unprocessable_content
    assert_select "#note_path_error", text: "Path is invalid"
  end

  test "show offers a delete button that asks for confirmation" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])

    get note_path("1")

    assert_select "form[action=?] input[name=_method][value=delete]", note_path("1")
    assert_select "header button[data-turbo-confirm]", text: "Delete"
  end

  test "destroy deletes the note and redirects home with a notice" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])

    delete note_path("1")

    assert_redirected_to root_path
    follow_redirect!
    assert_select "main", text: /Deleted/
    assert_select "nav li", count: 0
  end

  test "destroy of a note already deleted still redirects home" do
    use_fake_api(notes: [])

    delete note_path("gone")

    assert_redirected_to root_path
    assert_equal "Deleted", flash[:notice]
  end

  test "index answers bad gateway when the API is unreachable" do
    use_fake_api(unreachable: true)

    get root_path

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  test "update sends a CRLF note back with CRLF" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\r\ny\r\n" } ])

    patch note_path("1"), params: { note: { path: "a.md", content: "x\ny\nz\n" }, crlf: "true" }

    assert_equal "x\r\ny\r\nz\r\n", api.put_requests.sole[:content]
  end

  test "update keeps an LF note in LF when the browser sends CRLF" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\n" } ])

    patch note_path("1"), params: { note: { path: "a.md", content: "x\r\ny\r\n" }, crlf: "false" }

    assert_equal "x\ny\n", api.put_requests.sole[:content]
  end

  test "show records whether the note uses CRLF" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\r\n" } ])

    get note_path("1")

    assert_select "input[type=hidden][name=crlf][value=true]"
  end

  test "create with a path the API refuses keeps the text and flags the path" do
    use_fake_api(notes: [])

    post notes_path, params: { note: { path: " ", content: "mine" } }

    assert_response :unprocessable_content
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "#note_path_error", text: "Path is invalid"
  end
end
