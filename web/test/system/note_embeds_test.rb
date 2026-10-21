require "application_system_test_case"

# ![[nota]] na leitura embute a nota renderizada, como no Obsidian.
class NoteEmbedsTest < ApplicationSystemTestCase
  test "an embed shows the rendered note with a link to open it" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "Before\n\n![[ideas]]\n\nAfter\n" },
                          { id: "2", path: "ideas.md", content: "# Big idea\n\nSome **bold** text.\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview .note-embed h1", text: "Big idea"
    assert_selector ".note-preview .note-embed strong", text: "bold"
    within(".note-preview .note-embed") { assert_link "ideas", href: "/notes/2" }
    assert_no_text "![["
  end

  test "an embed inside an embed becomes a link" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "![[b]]\n" }, { id: "2", path: "b.md", content: "B body ![[c]]\n" },
                          { id: "3", path: "c.md", content: "C body\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview .note-embed", text: "B body"
    within(".note-preview .note-embed") { assert_link "c", href: "/notes/3" }
    assert_selector ".note-preview .note-embed", count: 1
    assert_no_text "C body"
  end

  # Ciclo: a nota que embute a si mesma usa o texto da página (até o não salvo) e não entra em laço.
  test "a note embedding itself shows its own unsaved text once" do
    use_fake_api(notes: [ { id: "1", path: "loop.md", content: "Self ![[loop]]\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Side by side"
    find(".cm-content").send_keys([ :control, :end ], " typed")

    assert_selector ".note-preview .note-embed", text: "Self loop typed", count: 1
    within(".note-preview .note-embed") { assert_link "loop", href: "/notes/1" }
  end

  test "an embed of a missing note shows the broken link warning" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "![[ghost]]\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview .wikilink-missing[title='No note named ghost']", text: "ghost"
    assert_no_selector ".note-preview .note-embed"
    assert_no_text "!ghost"
  end

  test "embedded content goes through the same sanitizer" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "![[evil]]\n" },
                          { id: "2", path: "evil.md", content: "Evil <img src=x onerror=\"window.pwned=1\"><script>window.pwned=2</script>\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview .note-embed img", visible: :all
    assert_no_selector ".note-preview .note-embed img[onerror]", visible: :all
    assert_no_selector ".note-preview .note-embed script", visible: :all
    assert_nil evaluate_script("window.pwned")
  end
end
