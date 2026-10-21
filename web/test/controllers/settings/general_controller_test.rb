require "test_helper"

# Configurações do tenant, seção General, como a General de um repositório no GitHub: o nome do tenant para todo
# papel e, no fim, a Danger zone só para o owner.
module Settings
  class GeneralControllerTest < ActionDispatch::IntegrationTest
    ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
    GENERAL = "/settings/general".freeze
    ACME = { id: "tenant-1", name: "Acme research", members: { ANA => "owner" } }.freeze

    def acme_as(role) = ACME.merge(members: { ANA => role })

    test "the general section shows the tenant name read only, with general current before members and invites" do
      use_fake_api(tenants: [ ACME ])
      sign_in_as

      get GENERAL

      assert_response :success
      assert_select "main h1", "General"
      assert_select "main dl#tenant_general dt", "Tenant name"
      assert_select "main dl#tenant_general dd", "Acme research"
      assert_select "main input[name*=name]", count: 0
      links = css_select("main nav a").map { it["href"] }
      assert_equal [ GENERAL, settings_members_path, settings_invites_path ], links.last(3)
      assert_select "main nav a[href=?][aria-current=page]", GENERAL, text: "General"
    end

    test "an owner sees the danger zone at the end, with the delete that asks for the tenant name" do
      use_fake_api(tenants: [ ACME ])
      sign_in_as

      get GENERAL

      assert_select "main section#danger_zone:last-child h2", "Danger zone"
      assert_select "#danger_zone", text: /deletes the tenant and all of its notes for every member/
      assert_select "#danger_zone form[action=?][method=post] input[name=_method][value=delete]", "/tenants/tenant-1"
      assert_select "#danger_zone button#delete_tenant.btn-danger[data-confirm-match=?][data-confirm-button=?]", "Acme research", "Delete tenant",
                    text: "Delete tenant"
      assert_select "#delete_tenant[data-turbo-confirm*=?]", "can't be undone"
    end

    test "an admin sees the name without the danger zone" do
      use_fake_api(tenants: [ acme_as("admin") ])
      sign_in_as

      get GENERAL

      assert_select "main dl#tenant_general dd", "Acme research"
      assert_select "#danger_zone", count: 0
      assert_select "#delete_tenant", count: 0
    end

    test "a member or viewer sees the general section alone among the tenant sections" do
      %w[member viewer].each do |role|
        use_fake_api(tenants: [ acme_as(role) ])
        sign_in_as

        get GENERAL

        assert_response :success, "expected the general section for a #{role}"
        assert_select "main dl#tenant_general dd", "Acme research"
        assert_select "#danger_zone", count: 0
        assert_select "main nav a[href=?]", GENERAL
        assert_select "main nav a[href=?]", settings_members_path, count: 0
        assert_select "main nav a[href=?]", settings_invites_path, count: 0
      end
    end

    test "without a tenant it sends to the tenant choice" do
      use_fake_api(tenants: [])
      sign_in_as

      get GENERAL

      assert_redirected_to tenants_path
    end

    test "answers bad gateway when the API is unreachable" do
      api = use_fake_api(tenants: [ ACME ])
      sign_in_as
      api.cut_connection!

      get GENERAL

      assert_response :bad_gateway
      assert_select "main", text: /Can't reach the markupp server/
    end
  end
end
