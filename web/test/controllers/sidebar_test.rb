require "test_helper"

# Barra lateral: cabeçalho do tenant com o seletor à parte e ícones das ações; conta, tema e configurações ficam no pé da
# barra de atividades, como no VS Code.
class SidebarTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "the tenant name leads home and a separate button opens the switcher" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav #current_tenant" do
      assert_select "a[href=?]", root_path, text: /Acme research/
      assert_select "button[popovertarget=tenant_switcher][aria-label=?][title=?] svg[data-icon=caret-up-down]", "Switch tenant", "Switch tenant"
    end
  end

  # De baixo para cima, como no VS Code: a engrenagem, a conta e o tema. O CSS esconde o botão do tema já ativo.
  test "the foot of the activity bar carries, bottom up, the settings, the account and the theme, without a sidebar footer" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav#sidebar footer", count: 0
    icons = css_select(".activity-bar-side .activity-bar-actions svg").map { it["data-icon"] }
    assert_equal %w[sun moon user-circle gear-six], icons
    assert_select ".activity-bar-side .activity-bar-actions" do
      assert_select "button[popovertarget=account_menu][aria-haspopup=menu][aria-label=Account][title=?]", EMAIL
      assert_select "a[href=?][aria-label=Settings][title=?][aria-keyshortcuts=?]", settings_path, "Settings (Ctrl+,)", "Control+, Meta+,"
    end
  end

  # A engrenagem já leva às configurações: o menu da conta só mostra quem está logado e o sair.
  test "the account icon opens the menu with the email and the log out" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "body > #account_menu[popover]:not([role])" do
      assert_select "[role=menu] [role=menuitem]", count: 1
      assert_select "form[action=?] button[role=menuitem]", session_path, text: "Log out"
      assert_select "p", text: /Signed in as\s*#{Regexp.escape(EMAIL)}/
    end
    assert_select "button", text: "Log out", count: 1
  end

  test "the activity bar shows the settings to a member too" do
    use_fake_api(notes: [], tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "member" } } ])
    sign_in_as

    get root_path

    assert_select ".activity-bar-side a[href=?][aria-label=Settings]", settings_path
  end

  test "the notes scroll in the sidebar, with the thin scrollbar of the theme" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    get root_path

    assert_select "nav > .sidebar-scroll.scrollbar-theme [role=treeitem]", text: "└── plan"
  end

  test "the theme toggle shows the sun or the moon of the theme it switches to" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select ".activity-bar-side form.theme-to-light button[aria-label=?][title=?] svg[data-icon=sun]", "Light theme", "Light theme"
    assert_select ".activity-bar-side form.theme-to-dark button[aria-label=?][title=?] svg[data-icon=moon]", "Dark theme", "Dark theme"
  end

  test "search carries its icon" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])
    sign_in_as

    get root_path

    assert_select "nav button[aria-keyshortcuts] svg[data-icon=magnifying-glass]"
  end

  test "save and delete carry icons" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    get note_path("1")

    assert_select "header button", text: "Save" do
      assert_select "svg[data-icon=floppy-disk]"
    end
    assert_select "header button[data-turbo-confirm]", text: "Delete" do
      assert_select "svg[data-icon=trash]"
    end
  end

  test "the sidebar starts open at its default width with a spine right after it to hide it and a resize handle" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "html[data-sidebar]", count: 0
    assert_select "html[style]", count: 0
    assert_select "button[aria-controls=sidebar]", count: 1
    assert_select "nav#sidebar + button[aria-controls=sidebar][aria-expanded=true][aria-label=?][title=?] svg[data-icon=sidebar-simple]",
                  "Hide sidebar", "Hide sidebar (Ctrl+B)"
    assert_select "nav#sidebar [role=separator][aria-orientation=vertical][tabindex='0'][aria-label=?]", "Resize sidebar" do |handle|
      assert_equal %w[240 480 288], %w[aria-valuemin aria-valuemax aria-valuenow].map { handle.first[it] }
    end
  end

  test "the closed sidebar comes back closed from its cookie" do
    use_fake_api(notes: [])
    sign_in_as
    cookies[:sidebar] = "closed"

    get root_path

    assert_select "html[data-sidebar=closed]"
    assert_select "button[aria-controls=sidebar]", count: 1
    assert_select "nav#sidebar + button[aria-controls=sidebar][aria-expanded=false][aria-label=?][title=?]", "Show sidebar", "Show sidebar (Ctrl+B)"
  end

  test "the sidebar width comes back from its cookie" do
    use_fake_api(notes: [])
    sign_in_as
    cookies[:sidebar_width] = "320"

    get root_path

    assert_select "html[style=?]", "--sidebar-width: 320px"
    assert_select "nav#sidebar [role=separator][aria-valuenow='320']"
  end

  test "the phone menu button and the sidebar are named after the notes on note pages" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "button[popovertarget=sidebar]", text: "Notes"
    assert_select "nav#sidebar[aria-label=?]", "Notes"
  end

  test "the phone menu button and the sidebar are named after the notes on settings pages, which show the tree" do
    use_fake_api(notes: [])
    sign_in_as

    get settings_account_path

    assert_select "button[popovertarget=sidebar]", text: "Notes"
    assert_select "nav#sidebar[aria-label=?]", "Notes"
  end

  test "the phone menu button and the sidebar are named after the settings on settings pages without tenant" do
    use_fake_api(notes: [], tenants: [])
    sign_in_as

    get settings_account_path

    assert_select "button[popovertarget=sidebar]", text: "Settings"
    assert_select "nav#sidebar[aria-label=?]", "Settings"
  end

  test "the phone menu button and the sidebar are named after the tenants on the onboarding" do
    use_fake_api(notes: [], tenants: [])
    sign_in_as

    get tenants_path

    assert_select "button[popovertarget=sidebar]", text: "Tenants"
    assert_select "nav#sidebar[aria-label=?]", "Tenants"
  end

  test "the activity bar beside the sidebar switches its view, the explorer by default" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    get root_path

    assert_select "body > .activity-bar-side + nav#sidebar"
    assert_select ".activity-bar-side > [role=tablist][aria-orientation=vertical][aria-label=?]", "Views"
    tabs = css_select(".activity-bar-side [role=tab]").map do |tab|
      [ tab["aria-label"], tab["title"], tab["aria-keyshortcuts"], tab["aria-controls"], tab["aria-selected"], tab.at_css("svg")["data-icon"] ]
    end
    assert_equal [ [ "Explorer", "Explorer (Ctrl+Shift+E)", "Control+Shift+E Meta+Shift+E", "sidebar_view_explorer", "true", "files" ],
                   [ "Links", "Links (Ctrl+Shift+L)", "Control+Shift+L Meta+Shift+L", "sidebar_view_links", "false", "link" ],
                   [ "Graph", "Graph (Ctrl+Shift+G)", "Control+Shift+G Meta+Shift+G", "sidebar_view_graph", "false", "graph" ] ], tabs
    assert_select ".activity-bar-side [role=tab][tabindex='0']", count: 1
    assert_select "nav #sidebar_view_explorer[role=tabpanel]:not([hidden]) [role=treeitem]", text: "└── plan"
    %w[links graph].each { assert_select "nav #sidebar_view_#{it}[role=tabpanel][hidden]" }
    # A busca fica no Ctrl+K e no botao do topo da barra lateral, sem visao propria.
    assert_select "#sidebar_view_search", count: 0
  end

  test "the chosen view comes back from its cookie, without the tree" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as
    cookies[:sidebar_view] = "links"

    get root_path

    assert_select ".activity-bar-side [role=tab][aria-selected=true]", count: 1
    assert_select ".activity-bar-side [role=tab][aria-label=Links][aria-selected=true][tabindex='0']"
    assert_select "#sidebar_view_links:not([hidden])", text: /Open a note to see its links/
    assert_select "#sidebar_view_explorer[hidden]"
  end

  test "the empty views say how to fill them" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "#sidebar_view_links", text: /Open a note to see its links/
    assert_select "#sidebar_view_graph", text: /Open a note to see its graph/
  end

  test "the phone drawer carries the same views in a row on its top" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav#sidebar > .activity-bar-drawer:first-child" do
      assert_select "[role=tablist][aria-orientation=horizontal] [role=tab]", count: 3
      assert_select "button[popovertarget=account_menu][aria-label=Account]"
      assert_select "a[href=?][aria-label=Settings]", settings_path
    end
  end

  test "without a tenant the activity bar keeps only the theme, the account and the settings" do
    use_fake_api(notes: [], tenants: [])
    sign_in_as

    get tenants_path

    assert_select "[role=tablist]", count: 0
    assert_select ".activity-bar-side button[aria-label=Account]"
    assert_select ".activity-bar-side a[href=?][aria-label=Settings]", settings_path
  end
end
