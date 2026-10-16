require "test_helper"

# Tenant ativo na sessão: escolha, troca pelo seletor da barra lateral e o cabeçalho X-Markupp-Tenant nas notas.
class TenantSelectionTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TWO_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                  { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ].freeze

  test "notes go to the API with the tenant of the session" do
    api = use_fake_api
    sign_in_as

    get root_path

    assert_response :success
    assert_equal "tenant-1", notes_tenant_sent(api)
  end

  test "with several tenants and none chosen the notes send to the tenant list" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get root_path

    assert_redirected_to tenants_path
  end

  test "the tenant list offers each tenant with its role" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get tenants_path

    assert_response :success
    assert_select "h1", "Choose a tenant"
    assert_select "main form[action=?]", current_tenant_path, count: 2
    assert_select "main form[action=?] input[name=tenant_id][value=tenant-2]", current_tenant_path
    assert_select "main form button", text: /Home lab\s+member/
  end

  test "choosing a tenant opens its notes with its header" do
    api = use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    patch current_tenant_path, params: { tenant_id: "tenant-2" }

    assert_redirected_to root_path
    get root_path
    assert_response :success
    assert_equal "tenant-2", notes_tenant_sent(api)
  end

  test "choosing a tenant outside the list is refused" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-1" }

    patch current_tenant_path, params: { tenant_id: "tenant-9" }

    assert_redirected_to tenants_path
    assert_equal "You no longer have access to that tenant.", flash[:alert]
    get root_path
    assert_response :success
  end

  test "a tenant lost since the last page falls back to the only one left" do
    api = use_fake_api(tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-2" }
    api.assign_role!("tenant-2", EMAIL, nil)

    get root_path

    assert_response :success
    assert_equal "tenant-1", notes_tenant_sent(api)
  end

  test "a tenant the API refuses in the middle of a page is forgotten" do
    api = use_fake_api(tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-2" }
    api.deny_notes_in!("tenant-2")

    get root_path

    assert_redirected_to tenants_path
    follow_redirect!
    assert_select "[role=alert]", "You no longer have access to that tenant."
    get root_path
    assert_redirected_to tenants_path
  end

  test "the sidebar shows the active tenant and its role in the switcher" do
    use_fake_api
    sign_in_as

    get root_path

    assert_select "nav button[popovertarget=tenant_switcher]", text: /markupp\s*\/\s*Acme research.*owner/m
  end

  test "the switcher lists every tenant with the active one marked" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-2" }

    get root_path

    assert_select "nav #tenant_switcher[popover]" do
      assert_select "form[action=?]", current_tenant_path, count: 2
      assert_select "button[aria-current=true]", count: 1, text: /Home lab\s+member/
      assert_select "button:not([aria-current])", text: /Acme research\s+owner/
      assert_select "a[href=?]", tenants_path, text: "New tenant"
    end
  end

  test "the tenant list answers bad gateway when the API is unreachable" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    get tenants_path

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end

  private
    def notes_tenant_sent(api) = api.received_credentials.reverse.find { it[:call] == "GET /notes" }[:tenant_id]
end
