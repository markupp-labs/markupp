require "test_helper"

# Configurações do cofre ativo, seção Members: a lista, adicionar um colega do tenant que ainda não é membro, remover e
# sair. O viewer só sai.
module Settings
  class VaultMembersControllerTest < ActionDispatch::IntegrationTest
    ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
    BIA = "bia@markupp.dev"
    CAIO = "caio@markupp.dev"
    ACME = { id: "tenant-1", name: "Acme research", members: { ANA => "member", BIA => "viewer", CAIO => "admin" } }.freeze
    VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ ANA, BIA ] },
               { id: "vault-2", tenant_id: "tenant-1", name: "Drafts", members: [ ANA ] } ].freeze

    def sign_in_with(role: "member", vaults: VAULTS)
      use_fake_api(tenants: [ ACME.merge(members: ACME[:members].merge(ANA => role)) ], vaults:)
      sign_in_as
    end

    test "the members of the vault show with their tenant role, the account marked with Leave vault" do
      sign_in_with

      get settings_vault_members_path

      assert_response :success
      assert_select "main h1", "Members"
      assert_select "main section h1 + p", "Research"
      rows = css_select("main ul#vault_members > li:not([aria-hidden])")
      assert_equal 2, rows.size
      assert_match(/#{ANA}\s*\(you\)/, rows.first.text)
      assert_select rows.first, "form[action=?] button", settings_vault_member_path("account-1"), text: "Leave vault"
      assert_select rows[1], "form[action=?] button", settings_vault_member_path("account-bia"), text: "Remove"
      assert_match(/viewer/, rows[1].text)
    end

    test "a writer adds a colleague of the tenant picked among the ones outside the vault" do
      sign_in_with

      get settings_vault_members_path

      assert_select "form#vault_member_add[action=?][method=post]", settings_vault_members_path do
        assert_equal [ [ "account-caio", CAIO ] ], css_select("select[name='member[user_id]'] option").map { [ it["value"], it.text ] }
        assert_select "button", text: "Add"
      end
    end

    test "with every colleague already in the vault the form gives way to a hint" do
      sign_in_with(vaults: [ VAULTS.first.merge(members: [ ANA, BIA, CAIO ]) ])

      get settings_vault_members_path

      assert_select "form#vault_member_add", count: 0
      assert_select "main", text: /Everyone in Acme research is already a member/
    end

    test "a viewer only gets Leave vault" do
      sign_in_with(role: "viewer")

      get settings_vault_members_path

      assert_select "form#vault_member_add", count: 0
      assert_select "main button", text: "Remove", count: 0
      assert_select "main button", text: "Leave vault", count: 1
    end

    test "add makes the colleague a member" do
      sign_in_with

      post settings_vault_members_path, params: { member: { user_id: "account-caio" } }

      assert_redirected_to settings_vault_members_path
      follow_redirect!
      assert_select "[role=status]", "Member added."
      assert_select "main ul#vault_members", text: /#{CAIO}/
    end

    test "add of a member or of someone outside the tenant shows why on top of the list" do
      sign_in_with

      post settings_vault_members_path, params: { member: { user_id: "account-bia" } }
      assert_response :conflict
      assert_select "main [role=alert]", "That person is already a member of this vault."

      post settings_vault_members_path, params: { member: { user_id: "account-9" } }
      assert_response :unprocessable_content
      assert_select "main [role=alert]", "Only people of this tenant can join its vaults. Invite them to the tenant first."
    end

    test "remove takes the member out of the vault" do
      sign_in_with

      delete settings_vault_member_path("account-bia")

      assert_redirected_to settings_vault_members_path
      follow_redirect!
      assert_select "[role=status]", "Member removed."
      assert_select "main ul#vault_members", text: /#{BIA}/, count: 0
    end

    test "remove of someone no longer in the vault says so" do
      sign_in_with

      delete settings_vault_member_path("account-caio")

      assert_response :not_found
      assert_select "main [role=alert]", "That person is no longer a member of this vault."
    end

    test "leaving the vault, even as a viewer, moves on to the next vault" do
      sign_in_with(role: "viewer", vaults: [ VAULTS.first, VAULTS.second.merge(members: [ ANA, BIA ]) ])

      delete settings_vault_member_path("account-1")

      assert_redirected_to root_path
      follow_redirect!
      assert_select "[role=status]", "You left Research."
      assert_select "button[popovertarget=vault_switcher]", text: "Drafts"
    end
  end
end
