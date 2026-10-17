require "test_helper"

# Barra lateral: cabeçalho do tenant com o seletor à parte, ícones das ações e rodapé com conta, configurações, tema e sair.
class SidebarTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "the tenant name leads home and a separate button opens the switcher" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav #current_tenant" do
      assert_select "a[href=?]", root_path, text: /markupp\s*\/\s*Acme research/
      assert_select "button[popovertarget=tenant_switcher][aria-label=?][title=?] svg[data-icon=caret-up-down]", "Switch tenant", "Switch tenant"
    end
  end

  test "the footer holds the account, the settings and the log out with icons" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav > footer" do
      assert_select "a[href=?] svg[data-icon=user-circle]", account_path
      assert_select "a[href=?]", account_path, text: EMAIL
      assert_select "a[href=?]", settings_path, text: "Settings" do
        assert_select "svg[data-icon=gear-six]"
      end
      assert_select "form[action=?] button", session_path, text: "Log out" do
        assert_select "svg[data-icon=sign-out]"
      end
    end
  end

  test "the footer hides the settings from a member" do
    use_fake_api(notes: [], tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "member" } } ])
    sign_in_as

    get root_path

    assert_select "nav > footer a[href=?]", settings_path, count: 0
  end

  test "the notes scroll apart from the fixed footer" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    get root_path

    assert_select "nav > .sidebar-scroll li", text: "plan.md"
    assert_select "nav > .sidebar-scroll footer", count: 0
  end

  test "the theme toggle shows the sun or the moon of the theme it switches to" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav footer form.theme-to-light button[aria-label=?][title=?] svg[data-icon=sun]", "Light theme", "Light theme"
    assert_select "nav footer form.theme-to-dark button[aria-label=?][title=?] svg[data-icon=moon]", "Dark theme", "Dark theme"
  end

  test "search, new note and the tree carry icons" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])
    sign_in_as

    get root_path

    assert_select "nav button[aria-keyshortcuts] svg[data-icon=magnifying-glass]"
    assert_select "nav a[href=?] svg[data-icon=file-plus]", new_note_path
    assert_select "nav summary svg[data-icon=folder]"
    assert_select "nav summary svg[data-icon=folder-open]"
    assert_select "nav a[href=?] svg[data-icon=file-text]", note_path("1")
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

  test "the sidebar starts open at its default width with a button to hide it and a resize handle" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "html[data-sidebar]", count: 0
    assert_select "html[style]", count: 0
    assert_select "nav#sidebar button[aria-controls=sidebar][aria-expanded=true][aria-label=?][title=?] svg[data-icon=sidebar-simple]",
                  "Hide sidebar", "Hide sidebar (Ctrl+B)"
    assert_select "main button[aria-controls=sidebar][aria-expanded=true][aria-label=?]", "Show sidebar"
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
    assert_select "button[aria-controls=sidebar][aria-expanded=false]", count: 2
  end

  test "the sidebar width comes back from its cookie" do
    use_fake_api(notes: [])
    sign_in_as
    cookies[:sidebar_width] = "320"

    get root_path

    assert_select "html[style=?]", "--sidebar-width: 320px"
    assert_select "nav#sidebar [role=separator][aria-valuenow='320']"
  end
end
