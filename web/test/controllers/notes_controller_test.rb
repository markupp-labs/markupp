require "test_helper"

class NotesControllerTest < ActionDispatch::IntegrationTest
  test "index shows the note tree in the sidebar" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" }, { id: "2", path: "inbox.md", content: "" } ])

    get root_path

    assert_response :success
    assert_select "nav[aria-label=?] details summary", "Notes", text: "work"
    assert_select "nav li", text: "plan.md"
    assert_select "nav li", text: "inbox.md"
  end

  test "index shows an empty state when there are no notes" do
    use_fake_api(notes: [])

    get root_path

    assert_response :success
    assert_select "nav", text: /No notes yet/
  end

  test "index answers bad gateway when the API is unreachable" do
    use_fake_api(unreachable: true)

    get root_path

    assert_response :bad_gateway
    assert_select "main", text: /Can't reach the markupp server/
  end
end
