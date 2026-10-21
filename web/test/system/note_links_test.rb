require "application_system_test_case"

# Visão Links da nota aberta: Ctrl+Shift+L leva o foco à primeira seção quando o frame chega, j e k andam pelas seções
# e linhas visíveis, Enter abre a nota ou dobra a seção, y copia o wikilink da sugestão, e a visão segue a nota aberta.
class NoteLinksTest < ApplicationSystemTestCase
  PLAN = { id: "plan", title: "plan", path: "work/plan.md" }.freeze
  ROADMAP = { id: "roadmap", title: "roadmap", path: "roadmap.md" }.freeze
  IDEAS = { id: "ideas", title: "ideas", path: "ideas.md" }.freeze
  LONG_CITED_TITLE = "Road map for the next quarters with every milestone the whole team agreed on".freeze
  QUARTERLY = { id: "quarterly", title: "Quarterly roadmap with every milestone agreed", path: "Quarterly roadmap with every milestone agreed.md" }.freeze
  NOTES = [ PLAN, ROADMAP, IDEAS ].map { { id: it[:id], path: it[:path], content: "# #{it[:title]}\n" } }.freeze
  GRAPH = {
    backlinks: { "plan" => [ { source: ROADMAP, target: PLAN, cited_title: "plan", type: "explicit", predicate: nil, inferred_target: nil } ],
                 "roadmap" => [ { source: IDEAS, target: ROADMAP, cited_title: "roadmap", type: "explicit", predicate: nil, inferred_target: nil } ] },
    links: { "plan" => [ { source: PLAN, target: nil, cited_title: "Ghost", type: "explicit", predicate: nil, inferred_target: nil },
                         { source: PLAN, target: nil, cited_title: LONG_CITED_TITLE, type: "explicit", predicate: nil, inferred_target: ROADMAP },
                         { source: PLAN, target: nil, cited_title: LONG_CITED_TITLE, type: "explicit", predicate: nil, inferred_target: QUARTERLY } ] },
    suggestions: { "plan" => [ { note: IDEAS, score: 0.03, resource_allocation: 0.5, cosine: 0.2 } ] }
  }.freeze

  setup do
    use_fake_api(notes: NOTES, graph: GRAPH)
    sign_in_through_form
    visit note_path("plan")
    assert_field "Path", with: "/work/plan"
  end

  test "ctrl+shift+l lands on the first section, j and k walk the rows, enter opens a backlink and the view follows" do
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_backlinks summary:focus"

    send_keys "j"
    assert_selector "#links_backlinks a:focus", text: "roadmap"
    send_keys "j"
    assert_selector "#links_outgoing summary:focus"
    send_keys "j"
    assert_selector "#links_outgoing .link-unresolved:focus", text: "Ghost"
    send_keys "k", "k", :enter

    assert_field "Path", with: "/roadmap"
    assert_selector "#links_backlinks a", text: "ideas"
  end

  test "enter on an unresolved link with an inferred note opens that note" do
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_backlinks summary:focus"

    send_keys "j", "j", "j", "j"
    assert_selector "#links_outgoing a.link-unresolved:focus", text: "maybe roadmap"
    send_keys :enter

    assert_field "Path", with: "/roadmap"
  end

  # Na linha estreita da barra lateral, só o título citado trunca: o maybe com a nota inferida fica inteiro.
  test "an unresolved link with a long cited title truncates the title and keeps the maybe whole" do
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_outgoing .links-row-maybe", text: "maybe roadmap"

    title, maybe = %w[title maybe].map do |part|
      evaluate_script("(e => [e.scrollWidth, e.clientWidth])(document.querySelector('#links_outgoing a.link-unresolved .links-row-#{part}'))")
    end

    assert_operator title[0], :>, title[1], "expected the cited title to truncate, its scroll and client widths are #{title}"
    assert_operator maybe[0], :<=, maybe[1], "expected the maybe to fit whole, its scroll and client widths are #{maybe}"
  end

  # Com o título da nota inferida também longo, o maybe trunca no lugar de empurrar a linha: as duas partes ficam à vista.
  test "an unresolved link with a long inferred title keeps both parts visible and the sidebar from scrolling sideways" do
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_outgoing .links-row-maybe", text: "maybe Quarterly roadmap"

    widths = %w[title maybe].to_h do |part|
      [ part, evaluate_script("document.querySelector('#links_outgoing a[href$=\"quarterly\"] .links-row-#{part}').clientWidth") ]
    end
    overflow = evaluate_script("(s => s.scrollWidth - s.clientWidth)(document.querySelector('.sidebar-scroll'))")

    assert_equal 0, overflow, "expected .sidebar-scroll not to scroll sideways, it overflows by #{overflow}px"
    assert widths.values.all? { it >= 40 }, "expected the cited title and the maybe to keep at least 40px each, got #{widths}"
  end

  # O texto só para leitor de tela da ligação não resolvida é absoluto: fora de uma linha posicionada, ele escapa para o
  # fim da barra lateral e a faz rolar inteira, com o cabeçalho junto. A janela baixa põe as linhas abaixo da dobra.
  test "the unresolved links keep the sidebar from scrolling as a whole" do
    page.current_window.resize_to(1400, 240)
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_outgoing .link-unresolved", text: "Ghost", visible: :all

    overflow = evaluate_script("(s => s.scrollHeight - s.clientHeight)(document.querySelector('nav#sidebar'))")

    assert_equal 0, overflow, "expected nav#sidebar not to scroll, it overflows by #{overflow}px"
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "enter folds a section and j skips its rows" do
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_backlinks summary:focus"

    send_keys :enter
    assert_no_selector "#links_backlinks[open]"
    send_keys "j"

    assert_selector "#links_outgoing summary:focus"
  end

  test "y on a suggestion and its button copy the wikilink" do
    page.driver.browser.execute_cdp("Browser.grantPermissions", origin: Capybara.current_session.server.base_url,
                                                                permissions: %w[clipboardReadWrite clipboardSanitizedWrite])
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_suggestions a", text: "ideas"
    find("#links_suggestions a", text: "ideas").execute_script("this.focus()")

    send_keys "y"
    assert_text "Copied [[ideas]]"
    assert_equal "[[ideas]]", evaluate_async_script("navigator.clipboard.readText().then(arguments[0])")
    find("#links_suggestions button[aria-label='Copy [[ideas]]']").click
    assert_text "Copied [[ideas]]"
  end

  test "alt+l goes from the activity bar into the links view" do
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_backlinks summary:focus"
    send_keys [ :alt, "h" ]
    assert_selector ".activity-bar-side [role=tab][aria-label=Links]:focus"

    send_keys [ :alt, "l" ]

    assert_selector "#links_backlinks summary:focus"
  end
end
