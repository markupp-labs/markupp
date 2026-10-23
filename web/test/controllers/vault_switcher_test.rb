require "test_helper"

# Trilha "tenant / cofre" no cabeçalho da barra lateral, cada parte com o seu seletor, e o cofre novo pelo diálogo.
class VaultSwitcherTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ EMAIL ] },
             { id: "vault-2", tenant_id: "tenant-1", name: "Drafts", members: [ EMAIL ] } ].freeze
  VIEWER = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "viewer" } } ].freeze

  test "the header is the trail of the tenant, small, and the vault, in focus, each opening its switcher" do
    use_fake_api(vaults: VAULTS)
    sign_in_as

    get root_path

    assert_select "nav #current_tenant" do
      assert_select "a[href=?][aria-label=Home] [data-mascot=small]", root_path
      assert_select "button[popovertarget=tenant_switcher][title=?][aria-label=?].text-xs", "Acme research", "Switch tenant: Acme research", text: "Acme research"
      assert_select "[aria-hidden=true]", text: "/"
      assert_select "button[popovertarget=vault_switcher][title=?][aria-label=?].font-semibold", "Research", "Switch vault: Research", text: "Research"
    end
  end

  test "the vault switcher lists the vaults as a menu, the active one marked, with New vault at the end" do
    use_fake_api(vaults: VAULTS)
    sign_in_as
    patch current_vault_path, params: { vault_id: "vault-2" }

    get root_path

    assert_select "#vault_switcher[popover][data-controller=context-menu] [role=menu]" do
      assert_select "form[action=?]", current_vault_path, count: 2
      assert_select "button[role=menuitem][aria-current=true]", count: 1, text: "Drafts"
      assert_select "button[role=menuitem]:not([aria-current])", text: "Research"
      assert_select "button[role=menuitem][commandfor=new_vault][command=show-modal]:last-child", text: /New vault/
    end
  end

  test "the new vault dialog posts a name of up to 100 characters" do
    use_fake_api(vaults: VAULTS)
    sign_in_as

    get root_path

    assert_select "dialog#new_vault[closedby=any] form[action=?][method=post]", vaults_path do
      assert_select "input[name=?][required][maxlength=100][autofocus]", "vault[name]"
      assert_select "button[formmethod=dialog]", text: "Cancel"
    end
  end

  test "without a vault the trail says so and the switcher still offers New vault" do
    use_fake_api(vaults: [])
    sign_in_as

    get root_path

    assert_select "button[popovertarget=vault_switcher]", text: "No vault"
    assert_select "#vault_switcher [role=menuitem]", count: 1, text: /New vault/
  end

  test "a viewer gets neither New vault nor its dialog" do
    use_fake_api(vaults: VAULTS, tenants: VIEWER)
    sign_in_as

    get root_path

    assert_select "#vault_switcher [role=menuitem][commandfor]", count: 0
    assert_select "dialog#new_vault", count: 0
  end

  test "create makes the vault, activates it and opens its empty tree" do
    use_fake_api(vaults: VAULTS, notes: [ { id: "1", path: "plan.md", content: "", vault_id: "vault-1" } ])
    sign_in_as

    post vaults_path, params: { vault: { name: " Field notes " } }

    assert_redirected_to root_path
    follow_redirect!
    assert_select "[role=status]", "Created Field notes."
    assert_select "button[popovertarget=vault_switcher]", text: "Field notes"
    assert_select "nav", text: /No notes yet/
  end

  test "create with a name the API refuses goes back with the rule" do
    use_fake_api(vaults: VAULTS)
    sign_in_as

    post vaults_path, params: { vault: { name: " " } }, headers: { "Referer" => "http://www.example.com/settings/account" }

    assert_redirected_to "/settings/account"
    assert_equal "Use 1 to 100 characters.", flash[:alert]
  end

  test "create by a viewer goes back with the read only alert" do
    use_fake_api(vaults: VAULTS, tenants: VIEWER)
    sign_in_as

    post vaults_path, params: { vault: { name: "Field notes" } }

    assert_redirected_to root_path
    assert_equal "Your role in this tenant is viewer, it can only read notes.", flash[:alert]
  end
end
