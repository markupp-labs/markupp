require "test_helper"

# Convites recebidos pela conta: listados no onboarding e no seletor de tenant, aceitos ou recusados dali.
class MyInvitesControllerTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  HOME_LAB = { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } }.freeze
  INVITE = { id: "invite-1", tenant_id: "tenant-2", email: EMAIL, role: "member", expires_at: "2099-10-23T12:00:00Z" }.freeze

  test "the onboarding lists the pending invites with accept and decline" do
    use_fake_api(tenants: [ HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    get tenants_path

    assert_select "h2", "Pending invites"
    assert_select "main li", text: /Join Home lab as member/ do
      assert_select "*", text: /Expires Oct 23/
      assert_select "form[action=?] input[name=_method][value=patch]", my_invite_path("invite-1")
      assert_select "form[action=?] button", my_invite_path("invite-1"), text: "Accept"
      assert_select "form[action=?] input[name=_method][value=delete]", my_invite_path("invite-1")
      assert_select "form[action=?] button", my_invite_path("invite-1"), text: "Decline"
    end
  end

  test "the onboarding says when invites show up when there are none" do
    use_fake_api(tenants: [])
    sign_in_as

    get tenants_path

    assert_select "h2", "Pending invites"
    assert_select "main", text: /No pending invites\. Invites show up here once your email is confirmed\./
  end

  test "an account with an unconfirmed email sees no invites" do
    use_fake_api(accounts: [ { email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, verified: false } ],
                 tenants: [ HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    get tenants_path

    assert_select "main", text: /Join Home lab/, count: 0
    assert_select "main", text: /No pending invites\./
  end

  test "accepting an invite joins the tenant and opens its notes" do
    api = use_fake_api(tenants: [ HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    patch my_invite_path("invite-1")

    assert_redirected_to root_path
    follow_redirect!
    assert_response :success
    assert_select "[role=status]", "Joined Home lab as member."
    assert_select "nav #current_tenant", text: /Home lab.*member/m
    assert_equal "tenant-2", api.received_credentials.reverse.find { it[:call].start_with?("GET /v1/vaults/") }[:tenant_id]
  end

  test "accepting an invite makes its tenant the active one among several" do
    use_fake_api(tenants: [ FakeMarkuppTenants::DEFAULT_TENANTS.sole, HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    patch my_invite_path("invite-1")
    follow_redirect!

    assert_select "nav #current_tenant", text: /Home lab.*member/m
  end

  test "declining an invite drops it and goes back" do
    use_fake_api(tenants: [ HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    delete my_invite_path("invite-1")

    assert_redirected_to tenants_path
    follow_redirect!
    assert_select "[role=status]", "Invite declined."
    assert_select "main", text: /Join Home lab/, count: 0
  end

  test "answering an invite that is no longer pending says so" do
    use_fake_api(tenants: [ HOME_LAB ])
    sign_in_as

    patch my_invite_path("invite-9")

    assert_redirected_to tenants_path
    follow_redirect!
    assert_select "[role=alert]", "That invite is no longer valid."
  end

  test "accepting an invite to a tenant already joined says so" do
    use_fake_api(invites: [ INVITE.merge(tenant_id: "tenant-1") ])
    sign_in_as

    patch my_invite_path("invite-1")

    assert_redirected_to tenants_path
    follow_redirect!
    assert_select "[role=alert]", "You are already a member of that tenant."
  end

  test "the tenant switcher loads the invites only when it opens" do
    use_fake_api
    sign_in_as

    get root_path

    assert_select "#tenant_switcher turbo-frame#my_invites[src=?][loading=lazy][target=_top]", my_invites_path, text: "" do
      assert_select "[data-controller=busy-spinner][data-busy-spinner-busy-when-value=?]", "#my_invites[aria-busy]"
    end
  end

  test "the invites frame lists the pending invites with accept and decline" do
    use_fake_api(tenants: [ FakeMarkuppTenants::DEFAULT_TENANTS.sole, HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    get my_invites_path, headers: { "Turbo-Frame" => "my_invites" }

    assert_response :success
    assert_select "turbo-frame#my_invites[target=_top] li", text: /Join Home lab as member/ do
      assert_select "button", "Accept"
      assert_select "button", "Decline"
    end
  end

  test "the invites frame is empty without invites" do
    use_fake_api
    sign_in_as

    get my_invites_path, headers: { "Turbo-Frame" => "my_invites" }

    assert_response :success
    assert_select "turbo-frame#my_invites", text: ""
  end

  test "the invites frame says when the API is unreachable" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    get my_invites_path, headers: { "Turbo-Frame" => "my_invites" }

    assert_response :bad_gateway
    assert_select "turbo-frame#my_invites", text: /Can't reach the markupp server/
  end

  test "accepting answers bad gateway when the API is unreachable" do
    api = use_fake_api(tenants: [ HOME_LAB ], invites: [ INVITE ])
    sign_in_as
    api.cut_connection!

    patch my_invite_path("invite-1")

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end
end
