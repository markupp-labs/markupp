require "test_helper"

# Recusar ligação pela visão Links e pelo grafo: a ligação automática sai das listas na hora; pela visão Links o frame
# recarrega com o aviso no status, pelo grafo o json traz o aviso. A recusa da API volta com o motivo traduzido.
class LinkRefusalsControllerTest < ActionDispatch::IntegrationTest
  PLAN = { id: "plan", title: "plan", path: "work/plan.md" }.freeze
  DRAFT = { id: "draft", title: "draft", path: "draft.md" }.freeze
  NOTES = [ { id: "plan", path: "work/plan.md", content: "" }, { id: "draft", path: "draft.md", content: "" },
            { id: "elsewhere", path: "elsewhere.md", content: "", vault_id: "vault-2" } ].freeze
  GRAPH = { backlinks: { "plan" => [ { source: DRAFT, target: PLAN, cited_title: "plan", type: "mentioned", predicate: nil, inferred_target: nil } ] } }.freeze
  REFUSED = "Link refused. It stays out until you undo it in the vault settings."
  FRAME = { "Turbo-Frame" => "note_links" }

  setup do
    use_fake_api(notes: NOTES, graph: GRAPH)
    sign_in_as
  end

  test "refusing from the links view reloads the frame without the row and announces it" do
    post link_refusals_path, params: { note_ids: %w[draft plan], note_id: "plan" }, headers: FRAME

    assert_redirected_to note_links_path("plan")
    follow_redirect!(headers: FRAME.dup)
    assert_select "#links_backlinks .links-none", text: "No backlinks yet"
    assert_select "[role=status][data-note-links-target=status][data-message=?]", REFUSED
  end

  test "a refusal the API turns down comes back to the links view with the reason" do
    post link_refusals_path, params: { note_ids: %w[plan elsewhere], note_id: "plan" }, headers: FRAME
    follow_redirect!(headers: FRAME.dup)
    assert_select "[role=status][data-message=?]", "One of these notes is no longer in this vault."

    post link_refusals_path, params: { note_ids: %w[plan plan], note_id: "plan" }, headers: FRAME
    follow_redirect!(headers: FRAME.dup)
    assert_select "[role=status][data-message=?]", "A note can't refuse a link with itself."
    assert_select "#links_backlinks a[href=?]", note_path("draft")
  end

  test "refusing from the graph answers json with the announcement or the reason" do
    post link_refusals_path(format: :json), params: { note_ids: %w[draft plan] }

    assert_response :created
    assert_equal REFUSED, response.parsed_body["message"]

    post link_refusals_path(format: :json), params: { note_ids: %w[plan elsewhere] }

    assert_response :unprocessable_content
    assert_equal "One of these notes is no longer in this vault.", response.parsed_body["message"]
  end

  test "a viewer that still had the action gets the reason, not the refusal" do
    use_fake_api(notes: NOTES, graph: GRAPH, tenants: FakeMarkuppTenants::VIEWER_TENANTS)
    sign_in_as

    post link_refusals_path(format: :json), params: { note_ids: %w[draft plan] }

    assert_response :forbidden
    assert_equal "Your role in this tenant is viewer, it can only read.", response.parsed_body["message"]
  end
end
