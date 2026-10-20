require "application_system_test_case"

# Barra de atividades como a do VS Code: uma coluna de ícones à esquerda da barra lateral troca a visão dela (Explorer,
# Links, Graph); a busca fica no Ctrl+K. Clicar na visão ativa recolhe a barra; com ela recolhida, clicar num ícone a abre naquela visão.
class ActivityBarTest < ApplicationSystemTestCase
  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form
  end

  test "a click switches the view of the sidebar and the view survives a reload" do
    visit root_path
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]"
    tab("Links").click

    assert_text "Open a note to see its links"
    assert_no_selector "[role=treeitem]"
    assert_selector "[role=tab][aria-label=Links][aria-selected=true]"
    refresh
    assert_text "Open a note to see its links"
    tab("Explorer").click
    assert_selector "[role=treeitem]", text: "plan"
  end

  test "a click on the active view hides the sidebar and a click on a view of the hidden sidebar opens it there" do
    visit root_path
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]"
    tab("Explorer").click
    assert_no_selector "nav#sidebar"
    assert_selector "button[aria-controls=sidebar][aria-expanded=false]"

    tab("Graph").click
    assert_selector "nav#sidebar"
    assert_text "Open a note to see its graph"
    assert_selector "button[aria-controls=sidebar][aria-expanded=true]"
  end

  test "the bar stays on the left edge, the sidebar beside it, also when the sidebar is hidden" do
    visit root_path
    assert_selector ".activity-bar-side"
    bar = find(".activity-bar-side")
    assert_equal [ 0, 48 ], [ edge(bar, "left"), edge(bar, "right") ].map(&:round)
    assert_in_delta 48, edge(find("nav#sidebar"), "left"), 0.5

    click_on "Hide sidebar"
    assert_no_selector "nav#sidebar"
    assert_in_delta 0, edge(bar, "left"), 0.5
    assert_operator edge(find("main"), "left"), :>=, 48
  end

  # No tamanho do VS Code: ícone de 24px num quadrado de 48px, a marca da visão ativa de 2px na borda de fora.
  test "the icons are 24px in 48px squares with the mark of the active view on the outer edge" do
    visit root_path
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]"
    explorer = tab("Explorer")

    assert_equal [ 48, 48 ], [ explorer.evaluate_script("this.offsetWidth"), explorer.evaluate_script("this.offsetHeight") ]
    assert_equal [ 24, 24 ], explorer.evaluate_script("[this.querySelector('svg').getBoundingClientRect().width, this.querySelector('svg').getBoundingClientRect().height]").map(&:round)
    assert_equal [ "0px", "2px" ], explorer.evaluate_script("['left', 'width'].map((name) => getComputedStyle(this, '::before')[name])")
  end

  test "the active view is marked only while the sidebar shows it" do
    visit root_path
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]"
    explorer = tab("Explorer")
    assert_equal css_color("var(--accent)"), explorer.evaluate_script("getComputedStyle(this, '::before').backgroundColor")

    click_on "Hide sidebar"
    assert_no_selector "nav#sidebar"
    assert_equal "0", explorer.evaluate_script("getComputedStyle(this, '::before').opacity")
  end

  # Como no VS Code: a engrenagem no fundo da coluna, a conta logo acima e o tema acima dela.
  test "the foot of the bar holds, bottom up, the settings, the account and the theme, and the account opens its menu" do
    visit root_path
    assert_selector ".activity-bar-side a[aria-label=Settings]"
    settings, account, theme = [ "a[aria-label=Settings]", "button[aria-label=Account]", "form:not([hidden]) button[aria-label$=theme]" ]
                                 .map { find(".activity-bar-side #{it}", match: :first) }
    assert_in_delta evaluate_script("innerHeight"), edge(settings, "bottom"), 4
    assert_operator edge(account, "bottom"), :<=, edge(settings, "top")
    assert_operator edge(theme, "bottom"), :<=, edge(account, "top")

    assert_equal [ [ 0, 48 ] ] * 3, [ settings, account, theme ].map { [ edge(it, "left"), edge(it, "right") ].map(&:round) }

    account.click
    within("#account_menu") { assert_text FakeMarkuppAccounts::DEFAULT_EMAIL }
    assert_operator edge(find("#account_menu"), "left"), :>=, 48
  end

  test "ctrl+shift+l and g show their views and ctrl+shift+e goes back to the tree" do
    visit note_path("1")
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]"

    find(".cm-content").send_keys([ :control, :shift, "l" ])
    assert_text "Open a note to see its links"
    assert_selector "#sidebar_view_links:focus"
    send_keys [ :control, :shift, "g" ]
    assert_selector "#sidebar_view_graph:focus"
    send_keys [ :control, :shift, "e" ]

    assert_selector "[role=treeitem][aria-current=page]:focus"
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]"
    assert_equal "# Plan\n", find("textarea", visible: :hidden).value
  end

  test "a view shortcut opens the hidden sidebar in that view" do
    visit root_path
    click_on "Hide sidebar"
    assert_no_selector "nav#sidebar"

    send_keys [ :control, :shift, "g" ]

    assert_selector "nav#sidebar"
    assert_selector "#sidebar_view_graph:focus"
  end

  test "j, k and the arrows walk the icons and enter shows the view" do
    visit root_path
    assert_selector "[role=tab][aria-label=Explorer][aria-selected=true]"
    tab("Explorer").execute_script("this.focus()")

    send_keys "j"
    assert_selector "[role=tab][aria-label=Links]:focus"
    send_keys :down, :down
    assert_selector "[role=tab][aria-label=Explorer]:focus"
    send_keys "k", :up
    assert_selector "[role=tab][aria-label=Links]:focus"
    send_keys :enter

    assert_text "Open a note to see its links"
    assert_selector "[role=tab][aria-label=Links][aria-selected=true]:focus"
  end

  test "alt+h goes from the tree to the bar and alt+l back" do
    visit note_path("1")
    send_keys [ :control, :shift, "e" ]
    assert_selector "[role=treeitem][aria-current=page]:focus"

    send_keys [ :alt, "h" ]
    assert_selector ".activity-bar-side [role=tab][aria-label=Explorer]:focus"
    send_keys [ :alt, "l" ]
    assert_selector "[role=treeitem][aria-current=page]:focus"
  end

  test "alt+l goes from the bar to an empty view" do
    visit root_path
    assert_selector "[role=tab][aria-label=Links]"
    tab("Links").click
    assert_text "Open a note to see its links"

    send_keys [ :alt, "l" ]

    assert_selector "#sidebar_view_links:focus"
  end

  test "on a phone the drawer shows the views in a row on its top" do
    page.current_window.resize_to(390, 844)
    visit root_path
    click_on "Notes"
    assert_selector "nav#sidebar .activity-bar-drawer [role=tab]", count: 3
    assert_no_selector ".activity-bar-side"

    find(".activity-bar-drawer [role=tab][aria-label=Graph]").click
    assert_text "Open a note to see its graph"
    assert_selector "nav#sidebar:popover-open"
  ensure
    page.current_window.resize_to(1400, 900)
  end

  private
    def tab(label)
      find(".activity-bar-side [role=tab][aria-label=#{label}]")
    end
end
