require "application_system_test_case"

# Visões do Explorer: o seletor (ou Alt+O, que antes abre o Explorer escondido) troca a árvore por listas planas do cofre sem sair da página, e as linhas
# andam por j/k, abrem o menu de contexto e recebem o Alt+h como a árvore.
class ExplorerModesTest < ApplicationSystemTestCase
  NOTES = [ { id: "hub", path: "work/hub.md", content: "# Hub\n", degree: 4, pagerank: 0.5, community: 2 },
            { id: "leaf", path: "leaf.md", content: "# Leaf\n", degree: 1, pagerank: 0.1, community: 2 },
            { id: "pair", path: "pair.md", content: "# Pair\n", degree: 2, pagerank: 0.3, community: 9 } ].freeze

  setup do
    use_fake_api(notes: NOTES)
    sign_in_through_form
    visit note_path("leaf")
    assert_field "Path", with: "/leaf"
  end

  test "choosing a view redraws the list in place, flat, with the open note marked and the tab stop on it" do
    select "Most linked", from: "Explorer view"

    assert_selector "[data-explorer-mode=linked]"
    assert_equal %w[hub pair leaf], all("[role=treeitem] .explorer-row-title").map(&:text)
    assert_selector "[role=treeitem][aria-current=page][tabindex='0']", text: "leaf"
    assert_selector "[role=treeitem][tabindex='0']", count: 1
    assert_field "Path", with: "/leaf"

    visit note_path("hub")
    assert_selector "[data-explorer-mode=linked] [role=treeitem][aria-current=page]", text: "hub"
  end

  test "the folder and the number of a flat row sit on the baseline of the title" do
    select "Most linked", from: "Explorer view"
    row = find("[data-explorer-mode=linked] [role=treeitem]", text: "hub")

    assert_equal "baseline", evaluate_script("getComputedStyle(arguments[0]).alignItems", row)
  end

  test "j and k walk the flat rows and enter opens one" do
    select "Most central", from: "Explorer view"
    assert_selector "[data-explorer-mode=central] [role=treeitem][tabindex='0']", text: "leaf"

    send_keys [ :control, :shift, "e" ]
    assert_selector "[role=treeitem]:focus", text: "leaf"
    send_keys "k"
    assert_selector "[role=treeitem]:focus", text: "pair"
    send_keys "k", "j", :enter

    assert_field "Path", with: "/pair"
  end

  test "a flat row opens the note menu and copies its path" do
    grant_clipboard
    select "By community", from: "Explorer view"
    find("[role=group] [role=treeitem]", text: "hub").right_click

    assert_selector "[role=menu] [role=menuitem]", text: "Rename / move"
    click_on "Copy path"

    assert_text "Copied /work/hub"
  end

  test "alt+o opens the view select and alt+h reaches the flat list from the editor" do
    find(".cm-content").click
    send_keys [ :alt, "o" ]
    # Com a lista de visões aberta, o foco fica na opção escolhida, dentro do select.
    assert_selector "select#explorer_mode option:focus", visible: :all
    send_keys :escape

    select "Most linked", from: "Explorer view"
    assert_selector "[data-explorer-mode=linked]"
    find(".cm-content").click
    send_keys [ :alt, "h" ]
    assert_selector "[role=treeitem][aria-current=page]:focus", text: "leaf"
  end

  test "alt+o opens the explorer first when the sidebar is hidden or shows another view" do
    click_on "Hide sidebar"
    assert_no_selector "nav#sidebar"
    find(".cm-content").click
    send_keys [ :alt, "o" ]
    assert_selector "select#explorer_mode option:focus", visible: :all
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]", match: :first
    send_keys :escape

    send_keys [ :control, :shift, "l" ]
    assert_selector "#sidebar_view_links #links_backlinks summary:focus"
    send_keys [ :alt, "o" ]

    assert_selector "select#explorer_mode option:focus", visible: :all
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]", match: :first
  end
end
