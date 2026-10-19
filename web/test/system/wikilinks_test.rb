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

  # Quebrado se distingue do link pelo tracejado e pela cor do texto (não a do link), sem ficar apagado; clicar não cria nota.
  test "a broken wikilink keeps the text color with a dashed underline in both themes and a click creates nothing" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "Ask [[nobody]] today.\n" } ])
    sign_in_through_form

    %w[Light Dark].each do |theme|
      visit settings_appearance_path
      within("main") { click_on theme }
      # O tema troca por um PATCH que volta para Aparência; abrir a nota antes disso perde a corrida para o redirect.
      assert_selector "main button[aria-pressed=true]", text: theme
      visit note_path("1")
      click_on "Read"
      assert_broken_link_style find(".note-preview .wikilink-missing", text: "nobody"), theme
    end
    find(".note-preview .wikilink-missing", text: "nobody").click

    assert_current_path note_path("1")
    assert_empty api.post_requests
  end

  # O title só aparece com mouse: para o leitor de tela o motivo vem no texto da página, escondido da vista.
  test "a broken wikilink says why without hover" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "Ask [[ghost|someone]].\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_includes find(".note-preview .wikilink-missing", text: "someone").text(:all), "No note named ghost"
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

  FILLER = Array.new(60) { |line| "Filler paragraph #{line}." }.join("\n\n")

  test "a wikilink to a heading opens the note scrolled to that heading" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "Go to [[deep#Deep Section]] or [[deep#Deep Section|the end]].\n" },
                          { id: "2", path: "deep.md", content: "# Top\n\n#{FILLER}\n\n## Deep Section\n\nThe end.\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    assert_link "the end", href: "/notes/2#deep-section"
    click_on "deep#Deep Section"

    assert_current_path note_path("2")
    assert_heading_in_view "deep-section"
  end

  test "a wikilink with only a heading scrolls within the note and repeated headings get their own ids" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "[[#Bottom part]]\n\n## Same\n\n#{FILLER}\n\n## Bottom part\n\n## Same\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    assert_selector "h2#user-content-same", text: "Same"
    assert_selector "h2#user-content-same-1", text: "Same"
    assert_link "#Bottom part", href: "#bottom-part"
    click_on "#Bottom part"

    assert_current_path note_path("1")
    assert_heading_in_view "bottom-part"
  end

  private
    # O id leva o prefixo do DOMPurify (SANITIZE_NAMED_PROPS), que impede o markdown de clobberar o DOM da página.
    # Dentro da janela inteira: perto do fim da nota o título não chega ao topo.
    def assert_heading_in_view(slug)
      assert_selector "#user-content-#{slug}" do |heading|
        heading.evaluate_script("(({ top, bottom }) => top >= 0 && bottom <= innerHeight)(this.getBoundingClientRect())")
      end
    end

    def assert_broken_link_style(element, theme)
      style = element.evaluate_script(<<~JS)
        (() => { const own = getComputedStyle(this), text = getComputedStyle(this.parentElement)
                 return { color: own.color, text: text.color, line: own.textDecorationLine, dash: own.textDecorationStyle } })()
      JS
      assert_equal style["text"], style["color"], "expected the broken link in the text color in #{theme}, got #{style}"
      assert_equal [ "underline", "dashed" ], style.values_at("line", "dash"), "expected a dashed underline in #{theme}, got #{style}"
    end
end
