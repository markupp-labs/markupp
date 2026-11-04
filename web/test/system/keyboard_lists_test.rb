require "application_system_test_case"

# j/k como no seletor de cofres (context_menu_controller.js): no popover de tenants e nas listas de membros do cofre e do
# tenant, nas configurações. Na lista, Delete na linha com o foco pede a confirmação de sempre para remover o membro.
class KeyboardListsTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  CAIO = "caio@markupp.dev"
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner", CAIO => "member" } },
              { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ].freeze
  VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ EMAIL, CAIO ] } ].freeze

  setup do
    use_fake_api(tenants: TENANTS, vaults: VAULTS)
    sign_in_through_form
    click_on "Acme research"
    assert_selector "nav #current_tenant", text: /Acme research/
  end

  test "the tenant switcher opens on the first tenant by the keyboard, j and k move and esc gives the focus back" do
    find("button[popovertarget=tenant_switcher]").send_keys(:enter)

    assert_selector "#tenant_switcher:popover-open [role=menuitem]:focus", text: "Acme research"
    send_keys "j"
    assert_selector "#tenant_switcher [role=menuitem]:focus", text: "Home lab"
    send_keys "k"
    assert_selector "#tenant_switcher [role=menuitem]:focus", text: "Acme research"
    send_keys :escape

    assert_no_selector "#tenant_switcher:popover-open"
    assert_selector "button[popovertarget=tenant_switcher]:focus"
  end

  test "in the vault members j and k move between the rows and delete asks to remove the member in focus" do
    visit settings_vault_members_path
    focus_first_row("#vault_members")

    send_keys "j"
    assert_selector "#vault_members li:focus", text: CAIO
    send_keys "k"
    assert_selector "#vault_members li:focus", text: EMAIL
    send_keys "j"
    accept_panel_confirm("Remove #{CAIO} from Research?", button: "Remove") { send_keys :delete }

    assert_text "Member removed."
    assert_no_selector "#vault_members", text: CAIO
  end

  test "in the tenant members j and k move between the rows and delete asks to remove the member in focus" do
    visit settings_members_path
    focus_first_row("#tenant_members")

    send_keys "j"
    assert_selector "#tenant_members li:focus", text: CAIO
    accept_panel_confirm("Remove #{CAIO} from Acme research?", button: "Remove") { send_keys :delete }

    assert_text "Member removed."
    assert_no_selector "#tenant_members", text: CAIO
  end

  private
    def focus_first_row(list)
      execute_script("document.querySelector(arguments[0]).focus()", "#{list} li[tabindex='0']")
      assert_selector "#{list} li:focus", text: EMAIL
    end
end
