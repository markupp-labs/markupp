require "test_helper"

# Configurações do tenant, aba Membros: só owner e admin entram; papel e remoção seguem o posto de quem age.
module Settings
  class MembersControllerTest < ActionDispatch::IntegrationTest
    ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
    BIA = "bia@markupp.dev"
    CAIO = "caio@markupp.dev"
    PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
    ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: BIA, password: PASSWORD } ].freeze
    ACME = { id: "tenant-1", name: "Acme research", members: { ANA => "owner", BIA => "admin", CAIO => "member" } }.freeze

    test "settings opens on the members tab" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get "/settings"

      assert_redirected_to settings_members_path
    end

    test "the members tab shows the tenant and both tabs with members current" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get settings_members_path

      assert_response :success
      assert_select "main h1", "Tenant settings"
      assert_select "main", text: /Acme research/
      assert_select "main nav a[href=?][aria-current=page]", settings_members_path, text: "Members"
      assert_select "main nav a[href=?]:not([aria-current])", settings_invites_path, text: "Invites"
    end

    test "the sidebar leads back to the notes and to both tabs" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get settings_members_path

      assert_select "nav#sidebar a[href=?]", root_path, text: "Back to notes"
      assert_select "nav#sidebar a[href=?]", settings_members_path, text: "Members"
      assert_select "nav#sidebar a[href=?]", settings_invites_path, text: "Invites"
    end

    test "a member or viewer is sent back to the notes" do
      use_fake_api(tenants: [ ACME.merge(members: { ANA => "member" }) ])
      sign_in_as

      get settings_members_path

      assert_redirected_to root_path
      assert_equal "Your role in this tenant can't do that.", flash[:alert]
    end

    test "the tenant switcher links to the settings for owner and admin" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as(BIA)

      get root_path

      assert_select "#tenant_switcher a[href=?]", settings_path, text: "Tenant settings"
    end

    test "the tenant switcher hides the settings from members" do
      use_fake_api(tenants: [ ACME.merge(members: { ANA => "member" }) ])
      sign_in_as

      get root_path

      assert_select "#tenant_switcher a[href=?]", settings_path, count: 0
    end

    test "lists each member with role, join date and the account marked as you" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get settings_members_path

      assert_select "main li", text: /#{ANA}\s*\(you\)/ do
        assert_select "*", text: /Sep 01/
      end
      assert_select "main li", text: /#{CAIO}/ do
        assert_select "*", text: /\(you\)/, count: 0
      end
    end

    test "an owner can give every role and remove anyone" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get settings_members_path

      assert_select "select#member_role_account-caio option", count: 4
      assert_select "select#member_role_account-caio option[selected]", "member"
      # Visível no celular, onde o cabeçalho das colunas some; só o desktop esconde.
      assert_select "label.md\\:sr-only:not(.sr-only)[for=member_role_account-caio]", "Role for #{CAIO}"
      assert_select "form[action=?] input[name=_method][value=patch]", settings_member_path("account-caio")
      assert_select "form[action=?] button[data-turbo-confirm=?]", settings_member_path("account-caio"), "Remove #{CAIO} from Acme research?", text: "Remove"
    end

    test "an admin gets roles up to admin and no controls on the owner" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as(BIA)

      get settings_members_path

      assert_equal %w[admin member viewer], css_select("select#member_role_account-caio option").map { it["value"] }
      assert_select "select#member_role_account-1", count: 0
      assert_select "form[action=?]", settings_member_path("account-1"), count: 0
      assert_select "main li", text: /#{ANA}.*owner/m
    end

    test "changing a role saves it and goes back to the list" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      patch settings_member_path("account-caio"), params: { member: { role: "viewer" } }

      assert_redirected_to settings_members_path
      follow_redirect!
      assert_select "[role=status]", "Role updated."
      assert_select "select#member_role_account-caio option[selected]", "viewer"
    end

    test "demoting the last owner says why inline" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      patch settings_member_path("account-1"), params: { member: { role: "admin" } }

      assert_response :conflict
      assert_select "main [role=alert]", "A tenant needs at least one owner. Make someone else owner first."
      assert_select "select#member_role_account-1 option[selected]", "owner"
    end

    test "touching a role above your own says why inline" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as(BIA)

      delete settings_member_path("account-1")

      assert_response :forbidden
      assert_select "main [role=alert]", "You can't give or change a role above your own."
    end

    test "changing someone who already left says so and reloads the list" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      patch settings_member_path("account-9"), params: { member: { role: "viewer" } }

      assert_response :not_found
      assert_select "main [role=alert]", "That person is no longer a member."
      assert_select "main li", text: /#{CAIO}/
    end

    test "removing a member drops them from the list" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      delete settings_member_path("account-caio")

      assert_redirected_to settings_members_path
      follow_redirect!
      assert_select "[role=status]", "Member removed."
      assert_select "main li", text: /#{CAIO}/, count: 0
    end

    test "removing yourself forgets the tenant and goes to the tenant list" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME.merge(members: { ANA => "owner", BIA => "owner" }) ])
      sign_in_as

      delete settings_member_path("account-1")

      assert_redirected_to tenants_path
      follow_redirect!
      assert_select "[role=status]", "You left Acme research."
      assert_select "h1", "Create your first tenant"
    end

    test "answers bad gateway when the API is unreachable" do
      api = use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as
      api.cut_connection!

      get settings_members_path

      assert_response :bad_gateway
      assert_select "main", text: /Can't reach the markupp server/
    end
  end
end
