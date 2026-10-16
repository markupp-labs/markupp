require "test_helper"

class SearchesControllerTest < ActionDispatch::IntegrationTest
  FRAME = { "Turbo-Frame" => "search_results" }.freeze
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TWO_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                  { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ].freeze

  test "show lists the matching notes in the results frame, linking out of it" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "Ship it" }, { id: "2", path: "inbox.md", content: "later" } ])
    sign_in_as

    get search_path(q: "ship")

    assert_response :success
    assert_select "turbo-frame#search_results a", count: 1
    assert_select "turbo-frame#search_results a[href=?][data-turbo-frame=_top]", note_path("1"), text: /plan\.md/ do
      assert_select ".text-ink-muted", text: "work"
    end
  end

  test "show selects the first result so Enter opens it" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "x" } ])
    sign_in_as

    get search_path(q: "x")

    assert_select "[role=option][aria-selected=true]", count: 1, text: "a.md"
    assert_select "[role=option][aria-selected=false]", count: 1, text: "b.md"
  end

  test "show says there are no matches" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    get search_path(q: "nothing")

    assert_response :success
    assert_select "turbo-frame#search_results", text: /No matches/
    assert_select "turbo-frame#search_results a", count: 0
  end

  test "show with an empty query renders an empty frame without calling the API" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    get search_path(q: "")

    assert_response :success
    assert_select "turbo-frame#search_results", text: ""
  end

  test "show says the server is unreachable inside the results frame" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    get search_path(q: "x")

    assert_response :bad_gateway
    assert_select "turbo-frame#search_results", text: /Can't reach the markupp server/
  end

  test "a tenant the API refuses during the search is forgotten with the notice inside the results frame" do
    api = use_fake_api(tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-2" }
    api.deny_notes_in!("tenant-2")

    get search_path(q: "x"), headers: FRAME

    assert_response :forbidden
    assert_frame_alert tenants_path, "You no longer have access to that tenant."
    get root_path
    assert_redirected_to tenants_path
  end

  test "a search without an active tenant says so inside the results frame" do
    use_fake_api(tenants: TWO_TENANTS)
    sign_in_as

    get search_path(q: "x"), headers: FRAME

    assert_response :forbidden
    assert_frame_alert tenants_path, "You no longer have access to that tenant."
  end

  test "an expired session during the search says so inside the results frame" do
    api = use_fake_api
    sign_in_as
    api.expire_access_tokens!
    api.revoke_refresh_tokens!

    get search_path(q: "x"), headers: FRAME

    assert_response :unauthorized
    assert_frame_alert new_session_path, "Your session expired. Log in again."
  end

  private
    # O link sai do frame: a página de destino não tem o frame da busca.
    def assert_frame_alert(path, message)
      assert_select "turbo-frame#search_results [role=alert] a[href=?][data-turbo-frame=_top]", path, text: message
    end
end
