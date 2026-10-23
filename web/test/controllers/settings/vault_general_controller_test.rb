require "test_helper"

# Configurações do cofre ativo, seção General, no grupo Vault acima do grupo Tenant: renomear e, na Danger zone, apagar
# confirmado pelo nome. O viewer só lê o nome.
module Settings
  class VaultGeneralControllerTest < ActionDispatch::IntegrationTest
    ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
    ACME = { id: "tenant-1", name: "Acme research", members: { ANA => "owner" } }.freeze
    VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ ANA ] },
               { id: "vault-2", tenant_id: "tenant-1", name: "Drafts", members: [ ANA ] } ].freeze

    def sign_in_with(role: "owner", vaults: VAULTS)
      use_fake_api(tenants: [ ACME.merge(members: { ANA => role }) ], vaults:)
      sign_in_as
    end

    test "the settings nav has the vault group, with its name, above the tenant group" do
      sign_in_with

      get settings_vault_path

      assert_response :success
      assert_select "main h1", "General"
      groups = css_select("main nav p.text-xs").map(&:text)
      assert_equal %w[Vault Tenant], groups
      assert_select "main nav p[title=?]", "Research"
      links = css_select("main nav a").map { it["href"] }
      assert_operator links.index(settings_vault_members_path), :<, links.index(settings_general_path)
      assert_select "main nav a[href=?][aria-current=page]", settings_vault_path, text: "General"
      assert_select "main nav a[href=?]", settings_vault_members_path, text: "Members"
    end

    test "a writer renames the vault in a form with its name" do
      sign_in_with(role: "member")

      get settings_vault_path

      assert_select "form#vault_rename[action=?][method=post]", vault_path("vault-1") do
        assert_select "input[name=_method][value=patch]", count: 1
        assert_select "input[name=?][value=?][required][maxlength=100]", "vault[name]", "Research"
      end
    end

    test "a writer gets the danger zone that deletes the vault once its name is typed" do
      sign_in_with(role: "member")

      get settings_vault_path

      assert_select "#danger_zone form[action=?]", vault_path("vault-1") do
        assert_select "button#delete_vault[data-confirm-match=?][data-confirm-match-param=confirm_name]", "Research", text: "Delete vault"
      end
    end

    test "a viewer reads the name with neither the rename form nor the danger zone" do
      sign_in_with(role: "viewer")

      get settings_vault_path

      assert_select "main dl#vault_general dd", "Research"
      assert_select "form#vault_rename", count: 0
      assert_select "#danger_zone", count: 0
    end

    test "without a vault the section sends to the empty state and the nav has no vault group" do
      sign_in_with(vaults: [])

      get settings_vault_path
      assert_redirected_to root_path
      assert_equal "Create a vault first, then add notes to it.", flash[:alert]

      get settings_account_path
      assert_select "main nav a[href=?]", settings_vault_path, count: 0
    end

    # O viewer não cria cofre: o aviso diz o que ele pode fazer, como o estado vazio (vaults/_empty).
    test "a viewer without a vault is told to ask for one, not to create it" do
      sign_in_with(role: "viewer", vaults: [])

      get settings_vault_path

      assert_redirected_to root_path
      assert_equal "Ask a member of a vault to add you.", flash[:alert]
    end

    test "rename gives the vault the new name" do
      sign_in_with

      patch vault_path("vault-1"), params: { vault: { name: "Papers" } }

      assert_redirected_to settings_vault_path
      follow_redirect!
      assert_select "[role=status]", "Renamed to Papers."
      assert_select "button[popovertarget=vault_switcher]", text: "Papers"
    end

    test "rename with a name the API refuses keeps the name and says the rule" do
      sign_in_with

      patch vault_path("vault-1"), params: { vault: { name: " " } }

      assert_redirected_to settings_vault_path
      assert_equal "Use 1 to 100 characters.", flash[:alert]
    end

    test "delete with the typed name removes the vault and moves on to the next one" do
      sign_in_with

      delete vault_path("vault-1"), params: { confirm_name: "Research" }

      assert_redirected_to root_path
      follow_redirect!
      assert_select "[role=status]", "Deleted Research."
      assert_select "button[popovertarget=vault_switcher]", text: "Drafts"
    end

    test "delete with another name deletes nothing" do
      sign_in_with

      delete vault_path("vault-1"), params: { confirm_name: "research" }

      assert_redirected_to settings_vault_path
      assert_equal "The name doesn't match. Nothing was deleted.", flash[:alert]
      get settings_vault_path
      assert_select "main nav p[title=?]", "Research"
    end

    test "delete of a vault out of reach says it is gone" do
      sign_in_with

      delete vault_path("vault-9"), params: { confirm_name: "Research" }

      assert_redirected_to root_path
      assert_equal "You no longer have access to that vault.", flash[:alert]
    end
  end
end
