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

    assert_select "nav", text: /#{FakeMarkuppAccounts::DEFAULT_EMAIL}/
    assert_select "nav button", text: "Search", count: 0
    assert_select "button[popovertarget=tenant_switcher]", count: 0
    assert_select "dialog", count: 0
  end

  test "the tenant list also offers a form for a new tenant" do
    use_fake_api
    sign_in_as

    get tenants_path

    assert_select "h1", "Choose a tenant"
    assert_select "h2", "New tenant"
    assert_select "main form[action=?][method=post] input[name=?]", tenants_path, "tenant[name]"
  end

  test "create makes the tenant, activates it and opens its notes" do
    api = use_fake_api(tenants: [])
    sign_in_as

    post tenants_path, params: { tenant: { name: "Acme research" } }

    assert_redirected_to root_path
    follow_redirect!
    assert_response :success
    assert_select "[role=status]", "Created Acme research."
    assert_select "nav button[popovertarget=tenant_switcher]", text: /Acme research.*owner/m
    assert_equal "tenant-1", api.received_credentials.reverse.find { it[:call] == "GET /notes" }[:tenant_id]
  end

  test "create with a blank name keeps the form and says the rule under the field" do
    use_fake_api(tenants: [])
    sign_in_as

    post tenants_path, params: { tenant: { name: "   " } }

    assert_response :unprocessable_content
    assert_select "input[name=?][aria-invalid=true][aria-describedby=tenant_name_error]", "tenant[name]"
    assert_select "#tenant_name_error", "Use 1 to 100 characters."
  end

  test "create answers bad gateway when the API is unreachable" do
    api = use_fake_api(tenants: [])
    sign_in_as
    api.cut_connection!

    post tenants_path, params: { tenant: { name: "Acme research" } }

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end
end
