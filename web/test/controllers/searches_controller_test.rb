require "test_helper"

class SearchesControllerTest < ActionDispatch::IntegrationTest
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
end
