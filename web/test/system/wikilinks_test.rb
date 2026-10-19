require "application_system_test_case"

class WikilinksTest < ApplicationSystemTestCase
  test "a wikilink to an existing note opens it, by name or by folder and name" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "See [[ideas]] and [[work/ideas]].\n" },
                          { id: "2", path: "ideas.md", content: "# Loose\n" }, { id: "3", path: "work/ideas.md", content: "# Work\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    assert_link "work/ideas", href: "/notes/3"
    click_on "ideas", exact: true

    assert_field "Title", with: "ideas"
    assert_current_path note_path("2")
  end

  test "an alias shows its text and links to the named note" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "Read [[ideas|the idea list]] first.\n" },
                          { id: "2", path: "ideas.md", content: "# Ideas\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_link "the idea list", href: "/notes/2"
    assert_no_text "[[ideas"
  end

  test "a wikilink to a missing note is marked as broken and links nowhere" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "Ask [[nobody]] and [[ghost|someone]].\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview .wikilink-missing[title='No note named nobody']", text: "nobody"
    assert_selector ".note-preview .wikilink-missing[title='No note named ghost']", text: "someone"
    assert_no_selector ".note-preview a", text: /nobody|someone/
  end

  test "a missing name with dollar signs keeps them in the title" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "Ask [[a$'b]] and [[c$&d]].\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_equal "No note named a$'b", find(".note-preview .wikilink-missing", text: "a$'b")["title"]
    assert_equal "No note named c$&d", find(".note-preview .wikilink-missing", text: "c$&d")["title"]
  end

  test "an ambiguous name picks the note in the same folder, else the shortest path" do
    use_fake_api(notes: [ { id: "1", path: "b/plan.md", content: "[[todo]]\n" }, { id: "2", path: "plan.md", content: "[[todo]]\n" },
                          { id: "3", path: "a/z/todo.md", content: "" }, { id: "4", path: "b/todo.md", content: "" },
                          { id: "5", path: "c/todo.md", content: "" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    assert_link "todo", href: "/notes/4"

    visit note_path("2")
    assert_link "todo", href: "/notes/4"
  end

  test "side by side turns a typed wikilink into a link" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" }, { id: "2", path: "ideas.md", content: "" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Side by side"
    find(".cm-content").send_keys("See [[Ideas]]")

    assert_link "Ideas", href: "/notes/2"
  end
end
