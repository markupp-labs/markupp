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

  test "the tenant list has a card per tenant, with its role, that enters it" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get tenants_path

    assert_response :success
    assert_select "h1", "Tenants"
    assert_select "main #tenant_cards > li", count: 2
    assert_select "main #tenant_cards > li", text: /Home lab\s+member/ do
      assert_select "form[action=?] input[name=tenant_id][value=tenant-2]", current_tenant_path
    end
  end

  test "the active tenant card is marked, like the current branch in git branch" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-2" }

    get tenants_path

    assert_select "#tenant_cards button[aria-current=true]", count: 1, text: /Home lab/
  end

  test "every card leads to the general settings by the gear, and only owner and admin cards to the invites by the envelope" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get tenants_path

    assert_select "#tenant_cards > li", text: /Acme research/ do
      assert_select "form:has(input[name=to][value=general]) button[aria-label=?][title=?] svg[data-icon=gear-six]",
                    "Acme research settings", "Acme research settings"
      assert_select "form:has(input[name=to][value=invites]) button[aria-label=?] svg[data-icon=envelope-simple]", "Invites of Acme research"
    end
    assert_select "#tenant_cards > li", text: /Home lab/ do
      assert_select "form:has(input[name=to][value=general]) button[aria-label=?] svg[data-icon=gear-six]", "Home lab settings"
      assert_select "input[name=to][value=invites]", count: 0
    end
  end

  test "entering a tenant from an icon of its card opens that page of the settings" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    patch current_tenant_path, params: { tenant_id: "tenant-1", to: "general" }
    assert_redirected_to settings_general_path
    patch current_tenant_path, params: { tenant_id: "tenant-1", to: "invites" }
    assert_redirected_to settings_invites_path
    patch current_tenant_path, params: { tenant_id: "tenant-1", to: "settings" }
    assert_redirected_to root_path
    patch current_tenant_path, params: { tenant_id: "tenant-1", to: "https://evil.example" }
    assert_redirected_to root_path
  end

  test "cards show the note stats when the API sends them" do
    use_fake_api(tenants: [
      { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" }, notes_count: 12, last_note_at: 2.hours.ago.utc.iso8601 },
      { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" }, notes_count: 1, last_note_at: 3.days.ago.utc.iso8601 },
      { id: "tenant-3", name: "Field notes", members: { EMAIL => "viewer" }, notes_count: 0, last_note_at: nil }
    ])
    sign_in_as

    get tenants_path

    assert_select "#tenant_cards > li", text: /Acme research.*12 notes, last edit 2h ago/m
    assert_select "#tenant_cards > li", text: /Home lab.*1 note, last edit 3d ago/m
    assert_select "#tenant_cards > li", text: /Field notes.*No notes yet/m
  end

  test "cards skip the stats line when the API sends no stats" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get tenants_path

    assert_select "#tenant_cards > li", count: 2
    assert_select "#tenant_cards", text: /notes?\b/, count: 0
  end

  test "pending invites come in a block above the cards, to accept or decline" do
    use_fake_api(tenants: TWO_TENANTS + [ { id: "tenant-3", name: "Field notes", members: { "bia@markupp.dev" => "owner" } } ],
                 invites: [ { id: "invite-1", tenant_id: "tenant-3", email: EMAIL, role: "member" } ])
    sign_in_as

    get tenants_path

    assert_select "main section#pending_invites ~ #tenant_cards"
    assert_select "#pending_invites", text: /Join Field notes as member/ do
      assert_select "form[action=?] button", my_invite_path("invite-1"), text: "Accept"
      assert_select "form[action=?] button", my_invite_path("invite-1"), text: "Decline"
    end
  end

  test "without pending invites the tenant list has no invites block" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get tenants_path

    assert_select "#pending_invites", count: 0
  end

  test "the header of the tenant list has the filter and the new tenant button, each with its key" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get tenants_path

    assert_select "main input[type=search][aria-label=?][aria-keyshortcuts=?][title=?]", "Filter tenants", "/", "Filter tenants (/)"
    assert_select "main button[aria-keyshortcuts=?][aria-haspopup=dialog][title=?]", "Alt+N", "New tenant (Alt+N)", text: /New tenant/
  end

  test "the new tenant address opens the tenant list with its dialog open" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get new_tenant_path

    assert_response :success
    assert_select "[data-tenant-grid-open-new-value=true] dialog#new_tenant"
  end

  test "the switcher ends with all the tenants, with its key" do
    use_fake_api
    sign_in_as

    get root_path

    assert_select "#tenant_switcher > :last-child a[href=?][title=?]", tenants_path, "All tenants (g t)", text: "All tenants"
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

    assert_select "nav #current_tenant", text: /Acme research\s*owner/
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
      assert_select "a[href=?]", new_tenant_path, text: "New tenant"
    end
  end

  test "a truncated tenant name shows whole on hover in the switcher and in the list" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-2" }

    get root_path

    assert_select "#current_tenant .truncate[title=?]", "Home lab"
    assert_select "#tenant_switcher button[title=?]", "Acme research"
    get tenants_path
    assert_select "main form button[title=?]", "Home lab"
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
    def notes_tenant_sent(api) = api.received_credentials.reverse.find { it[:call].start_with?("GET /v1/vaults/") }[:tenant_id]
end
