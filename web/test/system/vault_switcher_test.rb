require "application_system_test_case"

# Cofres pelo teclado: g e depois v abre o seletor de cofres com o foco no primeiro, j/k andam, Enter troca e Esc fecha;
# a paleta abre o New vault, e o diálogo cria o cofre já ativo.
class VaultSwitcherTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ EMAIL ] },
             { id: "vault-2", tenant_id: "tenant-1", name: "Drafts", members: [ EMAIL ] } ].freeze
  NOTES = [ { id: "1", path: "plan.md", content: "# Plan\n", vault_id: "vault-1" },
            { id: "2", path: "draft.md", content: "# Draft\n", vault_id: "vault-2" } ].freeze

  setup do
    use_fake_api(notes: NOTES, vaults: VAULTS)
    sign_in_through_form
    visit note_path("1")
    assert_field "Path", with: "/plan"
    execute_script("document.activeElement.blur()")
  end

  test "g then v opens the vault switcher on the first vault, j and k move and enter switches" do
    send_keys "g", "v"

    assert_selector "#vault_switcher:popover-open [role=menuitem]:focus", text: "Research"
    send_keys "j"
    assert_selector "#vault_switcher [role=menuitem]:focus", text: "Drafts"
    send_keys "k", "j", :enter

    assert_selector "#current_tenant button[popovertarget=vault_switcher]", text: "Drafts"
    assert_selector "[role=treeitem]", text: "draft"
    assert_no_selector "[role=treeitem]", text: "plan"
  end

  test "esc closes the vault switcher and gives the focus back to the vault name" do
    send_keys "g", "v"
    assert_selector "#vault_switcher:popover-open"

    send_keys :escape

    assert_no_selector "#vault_switcher:popover-open"
    assert_selector "#current_tenant button[popovertarget=vault_switcher]:focus"
  end

  test "the palette opens New vault, and the new vault becomes the active one" do
    send_keys [ :control, :shift, "p" ]
    send_keys "new vault", :enter

    assert_selector "dialog#new_vault[open] input:focus"
    send_keys "Field notes", :enter

    assert_selector "[role=status]", text: "Created Field notes."
    assert_selector "#current_tenant button[popovertarget=vault_switcher]", text: "Field notes"
  end
end
