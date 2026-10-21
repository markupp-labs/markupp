require "application_system_test_case"

# Apagar o tenant, como no GitHub: a Danger zone no fim da General do tenant, só para o owner, abre o diálogo do painel, que pede o
# nome exato do tenant antes de liberar o botão. Esc fecha sem apagar; apagado, o painel leva à lista de tenants.
class TenantDeletionTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner", "caio@markupp.dev" => "member" } },
              { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ].freeze

  setup do
    use_fake_api(tenants: TENANTS)
    sign_in_through_form
    visit tenants_path
    within("#tenant_cards > li", text: "Acme research") { click_on "Acme research" }
    visit "/settings/general"
    assert_selector "#danger_zone"
  end

  test "the dialog explains, focuses the name field and keeps delete off until the name matches exactly" do
    click_on "Delete tenant"

    within("dialog#confirm_dialog[open][aria-label='Delete tenant']") do
      assert_text "deletes the tenant and all of its notes for every member"
      assert_selector "label", text: "Type Acme research to confirm"
      assert_selector "input:focus"
      assert_selector "button.btn-danger[disabled]", text: "Delete tenant"
      send_keys "acme research"
      assert_selector "button.btn-danger[disabled]"
      find("input").fill_in(with: "Acme research")
      assert_selector "button.btn-danger:not([disabled])"
    end
  end

  test "esc closes the dialog and keeps the tenant" do
    click_on "Delete tenant"
    assert_selector "dialog#confirm_dialog[open] input:focus"
    send_keys "Acme research", :escape

    assert_no_selector "dialog#confirm_dialog[open]"
    assert_current_path "/settings/general"
    assert_selector "#delete_tenant:focus"
  end

  test "tab goes from the field to cancel and then to delete once the name matches, and enter deletes" do
    click_on "Delete tenant"
    send_keys "Acme research", :tab
    assert_selector "dialog#confirm_dialog[open] button:focus", text: "Cancel"
    send_keys :tab
    assert_selector "dialog#confirm_dialog[open] button.btn-danger:focus", text: "Delete tenant"
    send_keys :enter

    assert_current_path tenants_path
    assert_selector "[role=status]", text: "Deleted Acme research."
    assert_no_selector "#tenant_cards > li", text: "Acme research"
  end

  test "enter in the field deletes once the name matches" do
    click_on "Delete tenant"
    send_keys "Acme", :enter
    assert_selector "dialog#confirm_dialog[open]"
    send_keys " research", :enter

    assert_selector "[role=status]", text: "Deleted Acme research."
  end

  test "the command palette offers delete tenant on the general page and it opens the same dialog" do
    send_keys [ :control, :shift, "p" ]
    assert_selector "dialog#command_palette[open] input:focus"
    send_keys "delete tenant", :enter

    assert_selector "dialog#confirm_dialog[open] input:focus"
  end

  test "a later confirmation, after going to members, asks without the name field and with its button on" do
    click_on "Delete tenant"
    send_keys "Acme", :escape
    within("main nav") { click_on "Members" }
    assert_selector "h1", text: "Members"
    within("li", text: "caio@markupp.dev") { click_on "Remove" }

    within("dialog#confirm_dialog[open][aria-label=Remove]") do
      assert_no_selector "input"
      assert_selector "button.btn-danger:not([disabled])", text: "Remove"
      assert_selector "button:focus", text: "Cancel"
    end
  end
end
