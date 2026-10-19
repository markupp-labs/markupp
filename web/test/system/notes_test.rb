require "application_system_test_case"

class NotesTest < ApplicationSystemTestCase
  test "opening a note mounts the editor with its markdown" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n\nShip *it* with [[ideas]]." } ])
    sign_in_through_form

    visit root_path
    click_on "work"
    click_on "plan"

    assert_selector ".cm-editor .cm-content", text: "Ship *it* with [[ideas]]."
    assert_selector ".cm-wikilink", text: "[[ideas]]"
    assert_no_selector "textarea"
  end

  test "ctrl+s saves the edited note" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

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
    sign_in_through_form

    visit note_path("1")
    send_keys [ :alt, "n" ]
    assert_field "Path", with: "/work/"
    send_keys "Idea", :tab, "Fresh idea.", [ :control, "s" ]

    assert_text "Created"
    assert_selector "nav a[aria-current=page]", text: "Idea"
    assert_equal({ path: "work/Idea.md", content: "# Idea\n\nFresh idea." }, api.post_requests.sole)
  end

  test "renaming and moving the note marks it unsaved and saves the new path" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_field "Path", with: "/plan"
    fill_in "Path", with: "/work/roadmap"
    assert_text "Unsaved changes"
    click_on "Save"

    assert_text "Saved"
    assert_selector "nav a[aria-current=page]", text: "roadmap"
    assert_equal({ path: "work/roadmap.md", content: "# Plan\n" }, api.put_requests.sole.slice(:path, :content))
  end

  test "the home is a blank note: the path takes the focus and tab moves to the content" do
    api = use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    assert_field "Path", with: "/", focused: true
    send_keys "Plan"
    assert_selector ".cm-content", text: "Plan"
    send_keys :tab
    assert_selector ".cm-content:focus"
    send_keys "Ship it."
    send_keys [ :control, "s" ]

    assert_text "Created"
    assert_equal({ path: "Plan.md", content: "# Plan\n\nShip it." }, api.post_requests.sole)
  end

  test "shift+tab goes back from the content to the path" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    send_keys "Plan", :tab
    assert_selector ".cm-content:focus"
    send_keys [ :shift, :tab ]

    assert_field "Path", with: "/Plan", focused: true
  end

  test "the heading follows the title while the note is new" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    send_keys "Pla", :tab, "Body", [ :shift, :tab ], "n"

    assert_equal "# Plan\n\nBody", find("textarea", visible: :hidden).value
  end

  test "the heading takes only the name of the note, without the folders nor .md" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    send_keys "2026/Plan", :tab, "Body"
    assert_field "Path", with: "/2026/Plan"
    assert_equal "# Plan\n\nBody", find("textarea", visible: :hidden).value

    find_field("Path").send_keys([ :control, "a" ], "/abs/plan.md")
    assert_equal "# plan\n\nBody", find("textarea", visible: :hidden).value
  end

  test "a path refused for its trailing slash leaves a single heading once the name is typed again" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    send_keys "trail/", :tab, "Body."
    send_keys [ :control, "s" ]
    assert_text "Title can't be blank"
    assert_field "Path", with: "/trail/"
    find_field("Path").send_keys("Plan")

    assert_equal "# Plan\n\nBody.", find("textarea", visible: :hidden).value
  end

  test "a heading edited by hand is saved as typed, without the title one" do
    api = use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    send_keys "Baz", :tab, [ :control, :home ], :end, :backspace, :backspace, :backspace, "Foo", [ :control, :end ], "body"
    send_keys [ :control, "s" ]

    assert_text "Created"
    assert_equal({ path: "Baz.md", content: "# Foo\n\nbody" }, api.post_requests.sole)
  end

  test "after creation the title no longer touches the heading" do
    api = use_fake_api(notes: [ { id: "1", path: "Plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_field "Path", with: "/Plan"
    fill_in "Path", with: "/Roadmap"
    click_on "Save"

    assert_text "Saved"
    assert_equal({ path: "Roadmap.md", content: "# Plan\n" }, api.put_requests.sole.slice(:path, :content))
  end

  test "leaving the blank home untouched creates nothing and asks nothing" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit root_path
    send_keys [ :control, "s" ]
    click_on "plan"

    assert_current_path note_path("1")
    assert_empty api.post_requests
  end

  test "saving a new note without a name asks for one in place, keeping its folder and content" do
    api = use_fake_api(notes: [])
    sign_in_through_form

    visit new_note_path(folder: "work")
    execute_script("window.stayedOnPage = true")
    send_keys :tab, "Fresh idea.", [ :control, "s" ]

    assert_blank_title_refused_in_place
    assert_field "Path", with: "/work/"
    assert_selector ".cm-content", text: "Fresh idea."
    assert_empty api.post_requests
  end

  test "the blank title error goes away once a title is typed" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit new_note_path
    send_keys :tab, "Fresh idea.", [ :control, "s" ]
    assert_selector "#note_path_error", text: "Title can't be blank"
    send_keys "Plan"

    assert_no_selector "#note_path_error"
    assert_selector "#note_path:not([aria-invalid=true]):not([aria-describedby])"
  end

  # Com o foco já no título, devolver o foco não anuncia nada: o aviso precisa ser um alerta.
  test "ctrl+s typed in the blank title announces the refusal" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit new_note_path
    send_keys :tab, "Fresh idea.", [ :shift, :tab ], [ :control, "s" ]

    assert_selector "#note_path_error[role=alert]", text: "Title can't be blank"
  end

  test "saving a note with its path cleared asks for a name in place" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    execute_script("window.stayedOnPage = true")
    assert_field "Path", with: "/plan"
    fill_in "Path", with: ""
    find(".cm-content").click
    click_on "Save"

    assert_blank_title_refused_in_place
    assert_selector ".cm-content", text: "Plan"
    assert_empty api.put_requests
  end

  test "the save button shows up only once something changed" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_no_button "Save"
    find(".cm-content").send_keys([ :control, :end ], "!")
    assert_button "Save"
    find(".cm-content").send_keys(:backspace)

    assert_no_button "Save"
  end

  test "saving a CRLF note keeps its line breaks" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "a\r\nb\r\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_field "Path", with: "/plan"
    fill_in "Path", with: "renamed"
    click_on "Save"

    assert_text "Saved"
    assert_equal "a\r\nb\r\n", api.put_requests.sole[:content]
  end

  test "deleting a note with unsaved changes asks only once" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    find(".cm-content").send_keys([ :control, :end ], "draft")
    assert_text "Unsaved changes"
    accept_confirm("Delete this note? This cannot be undone.") { click_on "Delete" }

    assert_text "Deleted"
    assert_current_path root_path
  end

  test "an absolute path moves the note from the root and the field shows where it went" do
    api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_field "Path", with: "/work/plan"
    fill_in "Path", with: "/dev/teste.md"
    click_on "Save"

    assert_text "Saved"
    assert_field "Path", with: "/dev/teste"
    assert_equal "dev/teste.md", api.put_requests.sole[:path]
  end

  test "a relative path renames the note inside its folder and the field shows it absolute" do
    api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_field "Path", with: "/work/plan"
    fill_in "Path", with: "dev/teste"
    click_on "Save"

    assert_text "Saved"
    assert_field "Path", with: "/work/dev/teste"
    assert_equal "work/dev/teste.md", api.put_requests.sole[:path]
  end

  test "a path ending in a slash asks for a name in place and keeps what was typed" do
    api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_field "Path", with: "/work/plan"
    execute_script("window.stayedOnPage = true")
    fill_in "Path", with: "/dev/teste/"
    click_on "Save"

    assert_blank_title_refused_in_place
    assert_field "Path", with: "/dev/teste/"
    assert_empty api.put_requests
  end

  test "after deleting a note the blank note opens with the focus on its path" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    accept_confirm("Delete this note? This cannot be undone.") { click_on "Delete" }

    assert_text "Deleted"
    assert_field "Path", with: "/", focused: true
  end

  # O aviso acima do form não pode empurrar a nota em branco para fora da tela.
  test "after deleting a note the blank note fits the window with the notice above it" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    accept_confirm("Delete this note? This cannot be undone.") { click_on "Delete" }

    assert_text "Deleted"
    scroll, window = evaluate_script("[document.documentElement.scrollHeight, window.innerHeight]")
    assert_operator scroll, :<=, window, "expected nothing to scroll, got a page of #{scroll}px in a window of #{window}px"
  end

  test "a viewer gets the editor read only and ctrl+s does nothing" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ],
                       tenants: [ { id: "tenant-1", name: "Acme research", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "viewer" } } ])
    sign_in_through_form

    visit note_path("1")
    assert_selector ".cm-content[contenteditable=false]", text: "Plan"
    find("body").send_keys([ :control, "s" ])

    assert_text "Read only: your role here is viewer."
    assert_no_selector "[role=alert]"
    assert_empty api.received_credentials.select { it[:call] == "PUT /notes/1" }
  end

  private
    # Recusado no navegador: a página não recarregou e o foco voltou ao caminho.
    def assert_blank_title_refused_in_place
      assert_selector "#note_path_error", text: "Title can't be blank"
      assert_selector "#note_path[aria-invalid=true][aria-describedby=note_path_error]"
      assert_equal "note_path", evaluate_script("document.activeElement.id")
      assert evaluate_script("window.stayedOnPage"), "expected the page to stay, got a reload from the server"
    end
end
