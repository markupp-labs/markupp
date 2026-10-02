require "test_helper"

class SearchesControllerTest < ActionDispatch::IntegrationTest
  test "show lists the matching notes in the results frame, linking out of it" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "Ship it" }, { id: "2", path: "inbox.md", content: "later" } ])

    get search_path(q: "ship")

    assert_response :success
    assert_select "turbo-frame#search_results a", count: 1
    assert_select "turbo-frame#search_results a[href=?][data-turbo-frame=_top]", note_path("1"), text: /plan\.md/ do
      assert_select ".text-ink-muted", text: "work"
    end
  end

  test "show selects the first result so Enter opens it" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "x" } ])

    get search_path(q: "x")

    assert_select "[role=option][aria-selected=true]", count: 1, text: "a.md"
    assert_select "[role=option][aria-selected=false]", count: 1, text: "b.md"
  end

  test "show says there are no matches" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])

    get search_path(q: "nothing")

    assert_response :success
    assert_select "turbo-frame#search_results", text: /No matches/
    assert_select "turbo-frame#search_results a", count: 0
  end

  test "show with an empty query renders an empty frame without calling the API" do
    use_fake_api(unreachable: true)

    get search_path(q: "")

    assert_response :success
    assert_select "turbo-frame#search_results", text: ""
  end
end
