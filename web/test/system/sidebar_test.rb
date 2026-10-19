require "application_system_test_case"

class SidebarTest < ApplicationSystemTestCase
  test "the footer stays in view while a long note list scrolls" do
    use_fake_api(notes: (1..80).map { { id: it.to_s, path: "note-#{it}.md", content: "" } })
    sign_in_through_form

    visit root_path

    assert find("nav > .sidebar-scroll").evaluate_script("this.scrollHeight > this.clientHeight")
    footer_bottom = evaluate_script("document.querySelector('nav > footer').getBoundingClientRect().bottom")
    assert_operator footer_bottom, :<=, evaluate_script("window.innerHeight")
  end

  test "the tenant name leads home from a note" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    within("#current_tenant") { click_on "Acme research" }

    assert_current_path root_path
  end

  test "the tenant name fits beside the mascot at the default width" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path

    name = find("#current_tenant [title='Acme research']")
    assert name.evaluate_script("this.scrollWidth <= this.clientWidth"), "expected Acme research not to be cut at 288px"
  end

  test "a tenant name of twenty characters fits beside the mascot at the default width" do
    use_fake_api(notes: [], tenants: [ { id: "tenant-1", name: "Laboratorio de Dados", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "owner" } } ])
    sign_in_through_form

    visit root_path

    name = find("#current_tenant [title='Laboratorio de Dados']")
    assert name.evaluate_script("this.scrollWidth <= this.clientWidth"), "expected Laboratorio de Dados not to be cut at 288px"
  end

  test "one button in the content corner hides and shows the sidebar and the state survives a reload" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    assert_no_button "Show sidebar"
    within("main") { click_on "Hide sidebar" }
    assert_no_selector "nav#sidebar"
    assert_equal "Show sidebar (Ctrl+B)", find("main button[aria-controls=sidebar][aria-expanded=false]")["title"]
    within("main") { click_on "Show sidebar" }
    assert_selector "nav#sidebar"
    within("main") { click_on "Hide sidebar" }

    refresh
    assert_no_selector "nav#sidebar"
    within("main") { click_on "Show sidebar" }
    assert_selector "nav#sidebar"
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

    def sidebar_width
      evaluate_script("document.getElementById('sidebar').getBoundingClientRect().width")
    end
end
