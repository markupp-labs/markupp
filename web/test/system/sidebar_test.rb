require "application_system_test_case"

class SidebarTest < ApplicationSystemTestCase
  test "the settings at the foot of the activity bar stay in view while a long note list scrolls" do
    use_fake_api(notes: (1..80).map { { id: it.to_s, path: "note-#{it}.md", content: "" } })
    sign_in_through_form

    visit root_path

    assert find("nav > .sidebar-scroll").evaluate_script("this.scrollHeight > this.clientHeight")
    settings_bottom = evaluate_script("document.querySelector('.activity-bar-side a[aria-label=Settings]').getBoundingClientRect().bottom")
    assert_operator settings_bottom, :<=, evaluate_script("window.innerHeight")
  end

  test "the mascot leads home from a note and the tenant name opens the switcher" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    within("#current_tenant") { click_on "Switch tenant" }
    assert_selector "#tenant_switcher:popover-open"
    find("#current_tenant a[aria-label=Home]").click

    assert_current_path root_path
  end

  test "the trail of tenant and vault fits beside the mascot at the default width" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path

    [ "Acme research", "Notes" ].each do |name|
      part = find("#current_tenant [title='#{name}']")
      assert part.evaluate_script("this.scrollWidth <= this.clientWidth"), "expected #{name} not to be cut at 288px"
    end
  end

  # O cofre fica em destaque: um tenant longo é que encolhe, com o nome inteiro no title.
  test "a long tenant name gives way to the vault name, which stays whole" do
    tenants = [ { id: "tenant-1", name: "Laboratorio de Dados Abertos", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "owner" } } ]
    use_fake_api(notes: [], tenants:)
    sign_in_through_form

    visit root_path

    vault = find("#current_tenant [title='Notes']")
    assert vault.evaluate_script("this.scrollWidth <= this.clientWidth"), "expected Notes not to be cut at 288px"
    assert_selector "#current_tenant button[title='Laboratorio de Dados Abertos']"
  end

  test "the spine on the sidebar edge hides and shows it and the state survives a reload" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    assert_no_button "Show sidebar"
    click_on "Hide sidebar"
    assert_no_selector "nav#sidebar"
    assert_equal "Show sidebar (Ctrl+B)", find("button[aria-controls=sidebar][aria-expanded=false]")["title"]
    click_on "Show sidebar"
    assert_selector "nav#sidebar"
    click_on "Hide sidebar"

    refresh
    assert_no_selector "nav#sidebar"
    click_on "Show sidebar"
    assert_selector "nav#sidebar"
  end

  test "the spine sticks out of the sidebar edge near the top in its colors and beside the activity bar when closed" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    spine = find("button[aria-controls=sidebar]")
    sidebar = find("nav#sidebar")
    assert_in_delta edge(sidebar, "right") - 1, edge(spine, "left"), 0.5
    assert_operator edge(spine, "top"), :<, 40
    assert_equal style(sidebar, "backgroundColor"), style(spine, "backgroundColor")
    assert_equal style(sidebar, "borderRightColor"), style(spine, "borderTopColor")
    assert_equal "0px", style(spine, "borderLeftWidth")

    top = edge(spine, "top")
    spine.click
    assert_no_selector "nav#sidebar"
    assert_in_delta edge(find(".activity-bar-side"), "right") - 1, edge(spine, "left"), 0.5
    assert_in_delta top, edge(spine, "top"), 0.5
  end

  # Fechada, a barra some sem deixar faixa: a barra de atividades à vista já diz onde ela está, e o conteúdo começa logo
  # depois dela (32px).
  test "a closed sidebar leaves no strip and the content starts after the activity bar" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    click_on "Hide sidebar"
    assert_no_selector "nav#sidebar"

    assert_equal "none", strip_style("content")
    assert_in_delta 48, edge(find("main"), "left"), 0.5
  end

  # Hover e foco não pintam o fundo (a aba segue sendo a borda puxada da barra); só o ícone sobe para o accent, que
  # aparece também no claro (do ink-muted para o ink a diferença era 1.11:1).
  test "hovering or focusing the spine keeps the sidebar background and lifts only the icon to the accent" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    spine = find("button[aria-controls=sidebar]")
    spine.hover
    assert_equal style(find("nav#sidebar"), "backgroundColor"), style(spine, "backgroundColor")
    assert_equal css_color("var(--accent)"), style(spine, "color")

    find("body").hover
    spine.evaluate_script("this.focus()")
    assert spine.evaluate_script("this.matches(':focus-visible')")
    assert_equal css_color("var(--accent)"), style(spine, "color")
  end

  test "the spine leaves room around its icon" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    spine = find("button[aria-controls=sidebar]")
    icon = spine.find("svg", visible: :all)
    assert_operator edge(icon, "left") - edge(spine, "left"), :>=, 6
    assert_operator edge(spine, "right") - edge(icon, "right"), :>=, 6
  end

  test "the sidebar sticks to the top and the spine stays on its edge while a long note scrolls" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n\n#{(1..200).map { "Line #{it}." }.join("\n\n")}\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    assert_selector ".note-preview p", text: "Line 200."
    execute_script("window.scrollTo(0, 1500)")
    assert_operator evaluate_script("window.scrollY"), :>, 0

    sidebar = find("nav#sidebar")
    spine = find("button[aria-controls=sidebar]")
    assert_in_delta 0, edge(sidebar, "top"), 0.5
    assert_in_delta edge(sidebar, "right") - 1, edge(spine, "left"), 0.5
    assert_operator edge(spine, "top"), :<, 40
  end

  test "the resize handle stays reachable right beside the spine" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    y = edge(find("button[aria-controls=sidebar]"), "top") + 8
    right = edge(find("nav#sidebar"), "right")

    assert evaluate_script("document.elementFromPoint(#{right - 3}, #{y})?.matches('[role=separator]') == true")
    assert evaluate_script("document.elementFromPoint(#{right + 8}, #{y})?.closest('[aria-controls=sidebar]') != null")
  end

  test "ctrl+b toggles the sidebar with the focus in the editor and leaves the note untouched" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    find(".cm-content").send_keys([ :control, "b" ])
    assert_no_selector "nav#sidebar"
    find(".cm-content").send_keys([ :control, "b" ])

    assert_selector "nav#sidebar"
    assert_equal "# Plan\n", find("textarea", visible: :hidden).value
    assert_no_text "Unsaved changes"
  end

  test "dragging the edge resizes the sidebar and the width survives a reload" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    drag_by(find("[role=separator]"), 100)
    assert_in_delta 388, sidebar_width, 1

    refresh
    assert_in_delta 388, sidebar_width, 1
  end

  test "the arrow keys resize the sidebar from its handle" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    find("[role=separator]").send_keys(:right)

    assert_selector "[role=separator][aria-valuenow='304']"
    assert_in_delta 304, sidebar_width, 1
  end

  private
    def drag_by(element, dx)
      page.driver.browser.action.click_and_hold(element.native).move_by(dx, 0).release.perform
    end

    def style(element, property)
      element.evaluate_script("getComputedStyle(this).#{property}")
    end

    def strip_style(property)
      evaluate_script("getComputedStyle(document.body, '::before').#{property}")
    end

    def sidebar_width
      evaluate_script("document.getElementById('sidebar').getBoundingClientRect().width")
    end
end
