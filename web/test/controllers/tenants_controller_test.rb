require "test_helper"

# Onboarding: conta sem tenant cai na lista de tenants e cria o primeiro.
class TenantsControllerTest < ActionDispatch::IntegrationTest
  test "an account without tenants lands on the onboarding" do
    use_fake_api(tenants: [])
    sign_in_as

    get root_path
    assert_redirected_to tenants_path
    follow_redirect!

    assert_response :success
    assert_select "h1", "Create your first tenant"
    assert_select "main", text: /A tenant holds the notes you share with other people\./
    assert_select "main form[action=?][method=post]", tenants_path do
      assert_select "label[for=tenant_name]", "Name"
      assert_select "input[name=?][required][maxlength=100]", "tenant[name]"
      assert_select "button", "Create"
    end
  end

  test "the onboarding sidebar has no search nor tenant switcher" do
    use_fake_api(tenants: [])
    sign_in_as

    get tenants_path

    assert_select ".activity-bar-side button[aria-label=Account][title=?]", FakeMarkuppAccounts::DEFAULT_EMAIL
    assert_select "nav button", text: "Search", count: 0
    assert_select "button[popovertarget=tenant_switcher]", count: 0
    assert_select "dialog[data-search-target]", count: 0
  end

  test "the foot of the activity bar shows even without a tenant, with no view above it" do
    use_fake_api(tenants: [])
    sign_in_as

    get tenants_path

    assert_select ".activity-bar-side .activity-bar-actions a[aria-label=Settings]"
    assert_select ".activity-bar-side [role=tab]", count: 0
  end

  test "the tenant list also offers a form for a new tenant, in a dialog" do
    use_fake_api
    sign_in_as

    get tenants_path

    assert_select "h1", "Tenants"
    assert_select "dialog#new_tenant h2", "New tenant"
    assert_select "dialog#new_tenant form[action=?][method=post] input[name=?]", tenants_path, "tenant[name]"
  end

  test "create makes the tenant, activates it and opens its notes" do
    api = use_fake_api(tenants: [])
    sign_in_as

    post tenants_path, params: { tenant: { name: "Acme research" } }

    assert_redirected_to root_path
    follow_redirect!
    assert_response :success
    assert_select "[role=status]", "Created Acme research."
    assert_select "nav #current_tenant", text: /Acme research.*owner/m
    assert_equal "tenant-1", api.received_credentials.reverse.find { it[:call].start_with?("GET /v1/vaults/") }[:tenant_id]
  end

  test "create with a blank name keeps the form and says the rule under the field" do
    use_fake_api(tenants: [])
    sign_in_as

    post tenants_path, params: { tenant: { name: "   " } }

    assert_response :unprocessable_content
    assert_select "input[name=?][aria-invalid=true][aria-describedby=tenant_name_error]", "tenant[name]"
    assert_select "#tenant_name_error", "Use 1 to 100 characters."
  end

  test "create with a blank name next to other tenants reopens the dialog with the rule" do
    use_fake_api
    sign_in_as

    post tenants_path, params: { tenant: { name: "   " } }

    assert_response :unprocessable_content
    assert_select "[data-tenant-grid-open-new-value=true] dialog#new_tenant #tenant_name_error", "Use 1 to 100 characters."
  end

  test "create answers bad gateway when the API is unreachable" do
    api = use_fake_api(tenants: [])
    sign_in_as
    api.cut_connection!

    post tenants_path, params: { tenant: { name: "Acme research" } }

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  ACME = { id: "tenant-1", name: "Acme research", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "owner" } }.freeze
  HOME = { id: "tenant-2", name: "Home lab", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "member" } }.freeze
  LAB = { id: "tenant-3", name: "Field lab", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "owner" } }.freeze

  test "destroy by the owner deletes the tenant, forgets it and lands on the tenant list" do
    use_fake_api(tenants: [ ACME, HOME, LAB ])
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-1" }

    delete "/tenants/tenant-1", params: { confirm_name: "Acme research" }

    assert_redirected_to tenants_path
    follow_redirect!
    assert_select "[role=status]", "Deleted Acme research."
    assert_select "main #tenant_cards > li", count: 2
    assert_select "main #tenant_cards > li", text: /Acme research/, count: 0
    get root_path
    assert_redirected_to tenants_path
  end

  test "destroy with a name that is missing or not exactly the tenant's keeps the tenant without asking the API" do
    use_fake_api(tenants: [ ACME, HOME ])
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-1" }

    [ nil, "acme research", "Acme research " ].each do |typed|
      delete "/tenants/tenant-1", params: { confirm_name: typed }.compact, headers: { "Referer" => "http://www.example.com/settings/general" }

      assert_redirected_to "/settings/general"
      assert_equal "The name doesn't match. Nothing was deleted.", flash[:alert]
    end
    get tenants_path
    assert_select "main #tenant_cards > li", text: /Acme research/, count: 1
  end

  test "destroy by a role below owner is refused and keeps the tenant" do
    use_fake_api(tenants: [ ACME, HOME ])
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-2" }

    delete "/tenants/tenant-2", params: { confirm_name: "Home lab" }, headers: { "Referer" => "http://www.example.com/settings/general" }

    assert_redirected_to "/settings/general"
    assert_equal "Your role in this tenant can't do that.", flash[:alert]
    get tenants_path
    assert_select "main #tenant_cards > li", count: 2
  end

  test "destroy of a tenant that is already gone says so and forgets it" do
    api = use_fake_api(tenants: [ ACME, HOME, LAB ])
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-1" }
    api.delete_tenant!("tenant-1")

    delete "/tenants/tenant-1"

    assert_redirected_to tenants_path
    follow_redirect!
    assert_select "[role=alert]", "That tenant no longer exists."
  end

  test "destroy answers bad gateway when the API is unreachable" do
    api = use_fake_api(tenants: [ ACME ])
    sign_in_as
    api.cut_connection!

    delete "/tenants/tenant-1"

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end
end
