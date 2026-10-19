require "test_helper"

class NotesControllerTest < ActionDispatch::IntegrationTest
  test "index shows the note tree in the sidebar" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" }, { id: "2", path: "inbox.md", content: "" } ])
    sign_in_as

    get root_path

    assert_response :success
    assert_select "nav[aria-label=?] details summary", "Notes", text: "work"
    assert_select "nav li", text: "plan.md"
    assert_select "nav li", text: "inbox.md"
  end

  test "index shows an empty state when there are no notes" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_response :success
    assert_select "nav", text: /No notes yet/
  end

  test "index links each note of the tree to its page" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])
    sign_in_as

    get root_path

    assert_select "nav a[href=?]", note_path("1"), text: "plan.md"
    assert_select "nav details[open]", count: 0
  end

  test "show puts the exact markdown in the editor textarea" do
    content = "\n# Plan  \n\n\n- keep   \n\ttabs & <tags>\n\n"
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: } ])
    sign_in_as

    get note_path("1")

    assert_response :success
    assert_equal content, css_select("main textarea").sole.text
    assert_select "main input[name=?][value=?]", "note[title]", "plan"
    assert_select "main input[name=?][value=?]", "note[folder]", "work"
    assert_select "main input[name=?]:not([placeholder])", "note[folder]"
    assert_select "main input[name=?]:not([placeholder])", "note[title]"
    assert_select "main label[for=note_folder] + [aria-hidden=true]", "/"
  end

  test "show marks the current note and opens only its ancestors" do
    use_fake_api(notes: [ { id: "1", path: "work/2026/plan.md", content: "" }, { id: "2", path: "work/ideas.md", content: "" },
                          { id: "3", path: "home/list.md", content: "" } ])
    sign_in_as

    get note_path("1")

    assert_select "nav a[aria-current=page]", text: "plan.md", count: 1
    assert_select "nav a[aria-current]", count: 1
    assert_select "nav details[open] > summary", text: "work"
    assert_select "nav details[open] details[open] > summary", text: "2026"
    assert_select "nav details:not([open]) > summary", text: "home"
  end

  test "show answers not found for an unknown note" do
    use_fake_api(notes: [])
    sign_in_as

    get note_path("missing")

    assert_response :not_found
    assert_select "main", text: /Note not found/
  end

  # ![[nota]] na leitura busca o conteúdo da nota embutida por aqui, com a sessão, o tenant e o papel de quem lê.
  test "show as json gives the path and the content of the note" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n\nShip it.\n" } ])
    sign_in_as

    get note_path("1", format: :json)

    assert_response :success
    assert_equal({ "path" => "work/plan.md", "content" => "# Plan\n\nShip it.\n" }, response.parsed_body)
  end

  test "show as json answers not found for an unknown note without the page" do
    use_fake_api(notes: [])
    sign_in_as

    get note_path("missing", format: :json)

    assert_response :not_found
    assert response.body.empty?, "expected an empty json 404, got a #{response.media_type} body"
  end

  test "show as json without a session answers unauthorized and login does not go back to it" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])

    get note_path("1", format: :json)
    assert_response :unauthorized
    sign_in_as

    assert_redirected_to root_path
  end

  test "show as json with an expired session answers unauthorized and keeps the notice for the page" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as
    api.expire_access_tokens!
    api.revoke_refresh_tokens!

    get note_path("1", format: :json)

    assert_response :unauthorized
    assert_nil flash[:alert]
  end

  test "the save button names its shortcut like the mode buttons" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    get note_path("1")

    assert_select "button[title=?][aria-keyshortcuts=?]", "Save (Ctrl+S)", "Control+S Meta+S", text: "Save"
  end

  test "show puts the note in a save form with the version read" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x", updated_at: "2026-09-30T12:00:00.123Z" } ])
    sign_in_as

    get note_path("1")

    assert_select "form[action=?]", note_path("1") do
      assert_select "input[type=hidden][name=last_modified_at][value=?]", "2026-09-30T12:00:00.123Z"
      assert_select "input[type=text][name=?][value=?]", "note[title]", "a"
      assert_select "input[type=hidden][name=markdown][value=true]"
      assert_select "button[hidden]", text: "Save"
    end
  end

  test "update saves the note and redirects with a notice" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "old" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "", title: "a", content: "new" }, markdown: "true", last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_redirected_to note_path("1")
    assert_equal({ path: "a.md", content: "new", lastModifiedAt: "2026-09-30T12:00:00.123Z", force: false }, api.put_requests.sole)
    follow_redirect!
    assert_select "main", text: /Saved/
  end

  test "update on a conflict keeps the text and offers reload or overwrite" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)
    sign_in_as

    patch note_path("1"), params: { note: { folder: "", title: "a", content: "mine" }, markdown: "true", last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_response :conflict
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "button:not([hidden])", text: "Save"
    assert_select "input[name=last_modified_at][value=?]", "2026-09-30T12:00:00.123Z"
    assert_select "a[href=?]", note_path("1"), text: "Reload and discard my changes"
    assert_select "button[name=force][value=true]", text: "Overwrite"
  end

  test "update with force overwrites despite the conflict" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)
    sign_in_as

    patch note_path("1"), params: { note: { folder: "", title: "a", content: "mine" }, markdown: "true", last_modified_at: "2026-09-30T12:00:00.123Z", force: "true" }

    assert_redirected_to note_path("1")
    assert api.put_requests.sole[:force]
  end

  test "sidebar links new note to the folder of the open note" do
    use_fake_api(notes: [ { id: "1", path: "work/2026/plan.md", content: "" } ])
    sign_in_as

    get note_path("1")

    assert_select "nav a[href=?]", new_note_path(folder: "work/2026"), text: "New note"
  end

  test "sidebar links new note to the root when no note is open" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav a[href=?]", new_note_path, text: "New note"
  end

  test "new opens a blank note in the folder with the focus on an empty title" do
    use_fake_api(notes: [])
    sign_in_as

    get new_note_path(folder: "work")

    assert_response :success
    assert_select "form[action=?][method=post]", notes_path do
      assert_select "input[type=text][name=?][value=?]", "note[folder]", "work"
      assert_select "input[type=text][name=?][value=''][autofocus]:not([placeholder])", "note[title]"
      assert_select "button[hidden]", text: "Save"
    end
    assert_select "button", text: "Delete", count: 0
    assert_equal "", css_select("main textarea").sole.text
  end

  test "a new note is named New note in the browser tab until it is saved" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    get new_note_path
    assert_select "title", "New note"

    post notes_path, params: { note: { folder: "", title: "plan", content: "mine" } }
    assert_response :conflict
    assert_select "title", "New note"
  end

  test "the content of a new note is labelled Content, not by its unfinished path" do
    use_fake_api(notes: [])
    sign_in_as

    get new_note_path(folder: "work")

    assert_select "main textarea[aria-label=?]", "Content"
  end

  test "root opens a blank note at the top folder for a writer" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])
    sign_in_as

    get root_path

    assert_response :success
    assert_select "form[action=?][method=post]", notes_path do
      assert_select "input[name=?][value='']", "note[folder]"
      assert_select "input[name=?][value=''][autofocus]", "note[title]"
    end
    assert_equal "", css_select("main textarea").sole.text
    assert_select "nav a[aria-current]", count: 0
  end

  test "create posts the content as typed, the heading is the editor's job" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { folder: "work", title: "Plan", content: "Ship it." } }

    assert_redirected_to note_path("created-1")
    assert_equal({ path: "work/Plan.md", content: "Ship it." }, api.post_requests.sole)
  end

  test "create keeps a heading the user edited without adding the title one" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { folder: "", title: "Baz", content: "# Foo bar\n\nbody" } }

    assert_equal({ path: "Baz.md", content: "# Foo bar\n\nbody" }, api.post_requests.sole)
  end

  test "create with a slash in the title files the note in a subfolder" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { folder: "work", title: "2026/Plan", content: "x" } }

    assert_equal "work/2026/Plan.md", api.post_requests.sole[:path]
  end

  test "create without a title keeps what was typed and asks for a title under the field" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { folder: "work", title: "  ", content: "Ship it." } }

    assert_response :unprocessable_content
    assert_empty api.post_requests
    assert_select "input[name=?][value=''][aria-invalid=true][aria-describedby=note_path_error]", "note[title]"
    assert_select "input[name=?][value=?]", "note[folder]", "work"
    assert_select "#note_path_error", text: "Title can't be blank"
    assert_equal "Ship it.", css_select("main textarea").sole.text
  end

  test "create with a taken path keeps the text and flags the path" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    post notes_path, params: { note: { folder: "", title: "plan", content: "# plan\n\nmine" } }

    assert_response :conflict
    assert_equal "# plan\n\nmine", css_select("main textarea").sole.text
    assert_select "input[name=?][value=?][aria-invalid=true]", "note[title]", "plan"
    assert_select "#note_path_error", text: "Path already taken"
    assert_select "button:not([hidden])", text: "Save"
  end

  test "update renames the note and the tree shows the new name" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "", title: "b", content: "x" }, markdown: "true" }

    assert_redirected_to note_path("1")
    assert_equal "b.md", api.put_requests.sole[:path]
    follow_redirect!
    assert_select "nav a[aria-current=page]", text: "b.md"
    assert_select "nav li", text: "a.md", count: 0
  end

  test "update moves the note into a folder and opens it in the tree" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "work/2026", title: "a", content: "x" }, markdown: "true" }
    follow_redirect!

    assert_select "nav details[open] details[open] a[aria-current=page]", text: "a.md"
    assert_select "input[name=?][value=?]", "note[folder]", "work/2026"
  end

  test "update with a blank title keeps the name and asks for a title under the field" do
    api = use_fake_api(notes: [ { id: "1", path: "work/a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "work", title: "  ", content: "mine" }, markdown: "true" }

    assert_response :unprocessable_content
    assert_empty api.put_requests
    assert_select "input[name=?][value=''][aria-invalid=true][aria-describedby=note_path_error]", "note[title]"
    assert_select "input[name=?][value=?]", "note[folder]", "work"
    assert_select "#note_path_error", text: "Title can't be blank"
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "button:not([hidden])", text: "Save"
  end

  test "update on a taken path keeps what was typed and shows the error under the field" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "y" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "", title: "b", content: "mine" }, markdown: "true", last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_response :conflict
    assert_select "input[name=?][value=?][aria-invalid=true]", "note[title]", "b"
    assert_select "#note_path_error", text: "Path already taken"
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "input[name=last_modified_at][value=?]", "2026-09-30T12:00:00.123Z"
  end

  test "update on a path the API refuses shows its error under the field" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "../work", title: "a", content: "mine" }, markdown: "true" }

    assert_response :unprocessable_content
    assert_select "#note_path_error", text: "Path is invalid"
  end

  test "show offers a delete button that asks for confirmation" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    get note_path("1")

    assert_select "form[action=?] input[name=_method][value=delete]", note_path("1")
    assert_select "header button.btn-danger[data-turbo-confirm]", text: "Delete"
  end

  test "destroy deletes the note and redirects home with a notice" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    delete note_path("1")

    assert_redirected_to root_path
    follow_redirect!
    assert_select "main", text: /Deleted/
    assert_select "nav li", text: "a.md", count: 0
  end

  test "destroy of a note already deleted still redirects home" do
    use_fake_api(notes: [])
    sign_in_as

    delete note_path("gone")

    assert_redirected_to root_path
    assert_equal "Deleted", flash[:notice]
  end

  test "index answers bad gateway when the API is unreachable" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    get root_path

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  test "update sends a CRLF note back with CRLF" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\r\ny\r\n" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "", title: "a", content: "x\ny\nz\n" }, markdown: "true", crlf: "true" }

    assert_equal "x\r\ny\r\nz\r\n", api.put_requests.sole[:content]
  end

  test "update keeps an LF note in LF when the browser sends CRLF" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\n" } ])
    sign_in_as

    patch note_path("1"), params: { note: { folder: "", title: "a", content: "x\r\ny\r\n" }, markdown: "true", crlf: "false" }

    assert_equal "x\ny\n", api.put_requests.sole[:content]
  end

  test "show records whether the note uses CRLF" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\r\n" } ])
    sign_in_as

    get note_path("1")

    assert_select "input[type=hidden][name=crlf][value=true]"
  end

  test "update keeps a name without .md as it is" do
    api = use_fake_api(notes: [ { id: "1", path: "README", content: "x" } ])
    sign_in_as

    get note_path("1")
    assert_select "input[name=?][value=?]", "note[title]", "README"
    assert_select "input[type=hidden][name=markdown][value=false]"

    patch note_path("1"), params: { note: { folder: "", title: "README", content: "y" }, markdown: "false" }

    assert_equal "README", api.put_requests.sole[:path]
  end

  test "create with a path the API refuses keeps the text and flags the path" do
    use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { folder: "..", title: "a", content: "mine" } }

    assert_response :unprocessable_content
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "#note_path_error", text: "Path is invalid"
  end
end
