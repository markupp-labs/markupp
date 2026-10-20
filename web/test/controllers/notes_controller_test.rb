require "test_helper"

class NotesControllerTest < ActionDispatch::IntegrationTest
  test "index shows the note tree in the sidebar" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" }, { id: "2", path: "inbox.md", content: "" } ])
    sign_in_as

    get root_path

    assert_response :success
    assert_select "nav[aria-label=?] [role=tree] [role=treeitem][aria-expanded=false]", "Notes", text: "├── work/"
    assert_select "nav [role=treeitem]", text: "│ └── plan"
    assert_select "nav [role=treeitem]", text: "└── inbox"
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

    assert_select "nav a[href=?]", note_path("1"), text: "└── plan"
    assert_select "nav [role=group]:not([hidden])", count: 0
  end

  test "show puts the exact markdown in the editor textarea" do
    content = "\n# Plan  \n\n\n- keep   \n\ttabs & <tags>\n\n"
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: } ])
    sign_in_as

    get note_path("1")

    assert_response :success
    assert_equal content, css_select("main textarea").sole.text
    assert_select "main input[type=text][name=?][value=?]:not([placeholder])", "note[path]", "/work/plan"
    assert_select "main input[type=hidden][name=origin_path][value=?]", "work/plan.md"
    assert_select "main input[name=?]", "note[folder]", count: 0
    assert_select "main input[name=?]", "note[title]", count: 0
  end

  test "show marks the current note and opens only its ancestors" do
    use_fake_api(notes: [ { id: "1", path: "work/2026/plan.md", content: "" }, { id: "2", path: "work/ideas.md", content: "" },
                          { id: "3", path: "home/list.md", content: "" } ])
    sign_in_as

    get note_path("1")

    assert_select "nav a[aria-current=page]", text: "│ └── plan", count: 1
    assert_select "nav a[aria-current]", count: 1
    assert_select "nav [role=treeitem][aria-expanded=true]", text: "└── work/"
    assert_select "nav [role=treeitem][aria-expanded=true]", text: "├── 2026/"
    assert_select "nav [role=treeitem][aria-expanded=false]", text: "├── home/"
  end

  test "show answers not found for an unknown note" do
    use_fake_api(notes: [])
    sign_in_as

    get note_path("missing")

    assert_response :not_found
    assert_select "main h1", "404: Note not found"
    assert_select "main pre[data-mascot=large]"
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
      assert_select "input[type=text][name=?][value=?]", "note[path]", "/a"
      assert_select "input[type=hidden][name=origin_path][value=?]", "a.md"
      assert_select "input[name=markdown]", count: 0
      assert_select "button[hidden]", text: "Save"
    end
  end

  test "update saves the note and redirects with a notice" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "old" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "/a", content: "new" }, origin_path: "a.md", last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_redirected_to note_path("1")
    assert_equal({ path: "a.md", content: "new", lastModifiedAt: "2026-09-30T12:00:00.123Z", force: false }, api.put_requests.sole)
    follow_redirect!
    assert_select "main", text: /Saved/
  end

  test "update on a conflict keeps the text and offers reload or overwrite" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)
    sign_in_as

    patch note_path("1"), params: { note: { path: "/a", content: "mine" }, origin_path: "a.md", last_modified_at: "2026-09-30T12:00:00.123Z" }

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

    patch note_path("1"), params: { note: { path: "/a", content: "mine" }, origin_path: "a.md", last_modified_at: "2026-09-30T12:00:00.123Z", force: "true" }

    assert_redirected_to note_path("1")
    assert api.put_requests.sole[:force]
  end

  test "alt+n in the sidebar makes a new note in the folder of the open note, with no new note button" do
    use_fake_api(notes: [ { id: "1", path: "work/2026/plan.md", content: "" } ])
    sign_in_as

    get note_path("1")

    assert_select "nav [data-note-tree-new-note-url-value=?]", new_note_path(folder: "work/2026")
    assert_select "nav a", text: "New note", count: 0
  end

  test "alt+n in the sidebar makes a new note at the root when no note is open" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav [data-note-tree-new-note-url-value=?]", new_note_path
  end

  test "new opens a blank note with the path of its folder and the focus on it" do
    use_fake_api(notes: [])
    sign_in_as

    get new_note_path(folder: "work")

    assert_response :success
    assert_select "form[action=?][method=post]", notes_path do
      assert_select "input[type=text][name=?][value=?][autofocus]:not([placeholder])", "note[path]", "/work/"
      assert_select "input[type=hidden][name=origin_path][value=?]", "work/"
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

    post notes_path, params: { note: { path: "/plan", content: "mine" }, origin_path: "" }
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
      assert_select "input[name=?][value=?][autofocus]", "note[path]", "/"
    end
    assert_equal "", css_select("main textarea").sole.text
    assert_select "nav a[aria-current]", count: 0
  end

  test "create posts the content as typed, the heading is the editor's job" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { path: "/work/Plan", content: "Ship it." }, origin_path: "" }

    assert_redirected_to note_path("created-1")
    assert_equal({ path: "work/Plan.md", content: "Ship it." }, api.post_requests.sole)
  end

  test "create keeps a heading the user edited without adding the title one" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { path: "/Baz", content: "# Foo bar\n\nbody" }, origin_path: "" }

    assert_equal([ { path: "Baz.md", content: "# Foo bar\n\nbody" } ], api.post_requests)
  end

  test "create with a relative path files the note under the folder it came from" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { path: "2026/Plan", content: "x" }, origin_path: "work/" }

    assert_equal [ "work/2026/Plan.md" ], api.post_requests.map { it[:path] }
  end

  test "create without a name keeps what was typed and asks for a name under the field" do
    api = use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { path: "/dev/teste/", content: "Ship it." }, origin_path: "work/" }

    assert_response :unprocessable_content
    assert_empty api.post_requests
    assert_select "input[name=?][value=?][aria-invalid=true][aria-describedby=note_path_error]", "note[path]", "/dev/teste/"
    assert_select "input[type=hidden][name=origin_path][value=?]", "work/"
    assert_select "#note_path_error", text: "Name can't be blank"
    assert_equal "Ship it.", css_select("main textarea").sole.text
  end

  test "create with a taken path keeps the text and flags the path" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    post notes_path, params: { note: { path: "/plan", content: "# plan\n\nmine" }, origin_path: "" }

    assert_response :conflict
    assert_equal "# plan\n\nmine", css_select("main textarea").sole.text
    assert_select "input[name=?][value=?][aria-invalid=true]", "note[path]", "/plan"
    assert_select "#note_path_error", text: "Path already taken"
    assert_select "button:not([hidden])", text: "Save"
  end

  test "update renames the note and the tree shows the new name" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "b", content: "x" }, origin_path: "a.md" }

    assert_redirected_to note_path("1")
    assert_equal "b.md", api.put_requests.sole[:path]
    follow_redirect!
    assert_select "nav a[aria-current=page]", text: "└── b"
    assert_select "nav [role=treeitem]", text: "└── a", count: 0
  end

  test "update moves the note into a folder and opens it in the tree" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "/work/2026/a", content: "x" }, origin_path: "a.md" }
    assert_redirected_to note_path("1")
    follow_redirect!

    assert_select "nav [role=group]:not([hidden]) [role=group]:not([hidden]) a[aria-current=page]", text: "└── a"
    assert_select "input[name=?][value=?]", "note[path]", "/work/2026/a"
  end

  test "update with a trailing slash keeps the typed path and the folder it came from" do
    api = use_fake_api(notes: [ { id: "1", path: "work/a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "/dev/teste/", content: "mine" }, origin_path: "work/a.md" }

    assert_response :unprocessable_content
    assert_empty api.put_requests
    assert_select "input[name=?][value=?][aria-invalid=true][aria-describedby=note_path_error]", "note[path]", "/dev/teste/"
    assert_select "input[type=hidden][name=origin_path][value=?]", "work/a.md"
    assert_select "#note_path_error", text: "Name can't be blank"
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "button:not([hidden])", text: "Save"
  end

  test "update on a taken path keeps what was typed and shows the error under the field" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "y" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "b", content: "mine" }, origin_path: "a.md", last_modified_at: "2026-09-30T12:00:00.123Z" }

    assert_response :conflict
    assert_select "input[name=?][value=?][aria-invalid=true]", "note[path]", "b"
    assert_select "#note_path_error", text: "Path already taken"
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "input[name=last_modified_at][value=?]", "2026-09-30T12:00:00.123Z"
  end

  test "update on a path the API refuses shows its error under the field" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "../work/a", content: "mine" }, origin_path: "a.md" }

    assert_response :unprocessable_content
    assert_select "#note_path_error", text: "Path can't contain .. or pass 1024 characters"
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
    assert_select "nav [role=treeitem]", count: 0
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
    assert_select "main h1", "502: Can't reach the markupp server"
  end

  test "show as json answers bad gateway without the page when the API is unreachable" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as
    api.cut_connection!

    get note_path("1", format: :json)

    assert_response :bad_gateway
    assert response.body.empty?, "expected an empty json 502, got a #{response.media_type} body"
  end

  test "update sends a CRLF note back with CRLF" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\r\ny\r\n" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "/a", content: "x\ny\nz\n" }, origin_path: "a.md", crlf: "true" }

    assert_equal [ "x\r\ny\r\nz\r\n" ], api.put_requests.map { it[:content] }
  end

  test "update keeps an LF note in LF when the browser sends CRLF" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x\n" } ])
    sign_in_as

    patch note_path("1"), params: { note: { path: "/a", content: "x\r\ny\r\n" }, origin_path: "a.md", crlf: "false" }

    assert_equal [ "x\ny\n" ], api.put_requests.map { it[:content] }
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
    assert_select "input[name=?][value=?]", "note[path]", "/README"

    patch note_path("1"), params: { note: { path: "/README", content: "y" }, origin_path: "README" }

    assert_equal "README", api.put_requests.sole[:path]
  end

  test "create with a path the API refuses keeps the text and flags the path" do
    use_fake_api(notes: [])
    sign_in_as

    post notes_path, params: { note: { path: "../a", content: "mine" }, origin_path: "" }

    assert_response :unprocessable_content
    assert_equal "mine", css_select("main textarea").sole.text
    assert_select "#note_path_error", text: "Path can't contain .. or pass 1024 characters"
  end

  # :e do modo Vim (editor/vim_mode.js): o caminho resolve como o campo de caminho, a partir da nota aberta.
  test "index with a path opens the note at that path, relative to the folder of the origin" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" }, { id: "2", path: "ideas.md", content: "" } ])
    sign_in_as

    get notes_path(path: "plan", origin_path: "work/notes.md")
    assert_redirected_to note_path("1")

    get notes_path(path: "/ideas", origin_path: "work/notes.md")
    assert_redirected_to note_path("2")
  end

  test "index with the path of no note opens a new note with that path, not saved yet" do
    api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])
    sign_in_as

    get notes_path(path: "2026/goals", origin_path: "work/plan.md")

    assert_response :success
    assert_select "form[action=?][method=post]", notes_path do
      assert_select "input[name=?][value=?]", "note[path]", "/work/2026/goals"
      assert_select "input[type=hidden][name=origin_path][value=?]", "work/2026/goals.md"
    end
    assert_select "title", "New note"
    assert_empty api.post_requests
  end

  # :wq salva e sai para a nota padrao.
  test "create and update with quit go to the default note after saving" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    post notes_path, params: { note: { path: "/idea", content: "x" }, origin_path: "", quit: "true" }
    assert_redirected_to root_path

    patch note_path("1"), params: { note: { path: "/plan", content: "y" }, origin_path: "plan.md", quit: "true" }
    assert_redirected_to root_path
  end
end
