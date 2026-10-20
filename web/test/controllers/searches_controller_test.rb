require "test_helper"

class SearchesControllerTest < ActionDispatch::IntegrationTest
  FRAME = { "Turbo-Frame" => "search_results" }.freeze
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TWO_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                  { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ].freeze

  test "show lists the ranked hits with the title, the folder and the snippet, linking out of the frame" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "Ship it" }, { id: "2", path: "inbox.md", content: "later" } ])
    sign_in_as

    get search_path(q: "ship")

    assert_response :success
    assert_select "turbo-frame#search_results a", count: 1
    assert_select "turbo-frame#search_results a[href=?][data-turbo-frame=_top]", note_path("1") do
      assert_select ".search-hit-title", text: "plan"
      assert_select ".search-hit-folder", text: "work"
      assert_select ".search-hit-snippet mark", text: "Ship"
    end
  end

  test "show keeps the order of the server ranking" do
    use_fake_api(notes: [ { id: "1", path: "once.md", content: "ship" }, { id: "2", path: "thrice.md", content: "ship ship ship" } ])
    sign_in_as

    get search_path(q: "ship")

    assert_equal %w[thrice once], css_select("[role=option] .search-hit-title").map(&:text)
  end

  test "show escapes the note text of the snippet and only marks the matched term" do
    use_fake_api(notes: [ { id: "1", path: "x.md", content: "<b>ship</b> & co" } ])
    sign_in_as

    get search_path(q: "ship")

    assert_select ".search-hit-snippet b", count: 0
    assert_select ".search-hit-snippet mark", count: 1, text: "ship"
    assert_includes css_select(".search-hit-snippet").first.text, "<b>"
  end

  test "show selects the first result so Enter opens it, the only one in the Tab order" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x x" }, { id: "2", path: "b.md", content: "x" } ])
    sign_in_as

    get search_path(q: "x")

    assert_select "[role=option][aria-selected=true][tabindex='0'] .search-hit-title", count: 1, text: "a"
    assert_select "[role=option][aria-selected=false][tabindex='-1'] .search-hit-title", count: 1, text: "b"
  end

  test "show says there are no matches" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    sign_in_as

    get search_path(q: "nothing")

    assert_response :success
    assert_select "turbo-frame#search_results", text: /No matches for "nothing"/
    assert_select "turbo-frame#search_results", text: /Try fewer words/
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
