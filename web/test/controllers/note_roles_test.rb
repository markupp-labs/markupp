require "test_helper"

# Papel no tenant ativo: viewer só lê, e a recusa 403 da API vira aviso.
class NoteRolesTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  NOTES = [ { id: "1", path: "plan.md", content: "# Plan\n" } ].freeze
  READ_ONLY_ALERT = "Your role in this tenant is viewer, it can only read notes."

  test "a viewer gets no new note link in the sidebar" do
    use_fake_api(notes: NOTES, tenants: viewer_tenant)
    sign_in_as

    get root_path

    assert_response :success
    assert_select "nav a", text: "New note", count: 0
    assert_select "main form", count: 0
    assert_select "main", text: /Pick a note from the sidebar/
  end

  test "a viewer reads the note without save nor delete" do
    use_fake_api(notes: NOTES, tenants: viewer_tenant)
    sign_in_as

    get note_path("1")

    assert_response :success
    assert_select "input[name=?][readonly]", "note[path]"
    assert_select "textarea[readonly]", text: /# Plan/
    assert_select "button", text: "Save", count: 0
    assert_select "button", text: "Delete", count: 0
    assert_select "form#delete_note", count: 0
    assert_select "main", text: /Read only: your role here is viewer\./
  end

  test "a writer still edits the note" do
    use_fake_api(notes: NOTES)
    sign_in_as

    get note_path("1")

    assert_select "textarea[readonly]", count: 0
    assert_select "button[hidden]", text: "Save"
    assert_select "main", text: /Read only/, count: 0
  end

  test "new sends a viewer back home with the read only alert without asking for the notes" do
    api = use_fake_api(notes: NOTES, tenants: viewer_tenant)
    sign_in_as

    get new_note_path

    assert_redirected_to root_path
    assert_equal READ_ONLY_ALERT, flash[:alert]
    assert_not api.received_credentials.any? { it[:call] == "GET /notes" }
  end

  test "a save refused because the role changed shows the alert and reloads the note" do
    api = use_fake_api(notes: NOTES)
    sign_in_as
    api.assign_role!("tenant-1", EMAIL, "viewer")

    patch note_path("1"), params: { note: { path: "plan", content: "mine" }, origin_path: "plan.md" }, headers: { "HTTP_REFERER" => note_url("1") }

    assert_redirected_to note_url("1")
    assert_empty api.put_requests
    follow_redirect!
    assert_select "[role=alert]", READ_ONLY_ALERT
    assert_select "textarea[readonly]", text: /# Plan/
  end

  test "a create refused for a viewer goes home with the alert" do
    api = use_fake_api(notes: NOTES)
    sign_in_as
    api.assign_role!("tenant-1", EMAIL, "viewer")

    post notes_path, params: { note: { path: "/new", content: "" }, origin_path: "" }

    assert_redirected_to root_path
    assert_equal READ_ONLY_ALERT, flash[:alert]
    assert_empty api.post_requests
  end

  private
    def viewer_tenant = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "viewer" } } ]
end
