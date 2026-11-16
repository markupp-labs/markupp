require "test_helper"

# Grafo da página pública (ADR-0048): o canvas do painel, com os nós da publicação pelo id público e o link de cada um.
class PublicGraphsControllerTest < ActionDispatch::IntegrationTest
  NOTES = [
    { id: "start", path: "docs/start.md", content: "# Start\n" },
    { id: "setup", path: "docs/setup.md", content: "# Setup\n" },
    { id: "secret", path: "secret.md", content: "# Secret\n" }
  ].freeze
  START = FakeMarkuppPublications.public_id_for("start")
  SETUP = FakeMarkuppPublications.public_id_for("setup")

  setup do
    use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])
  end

  test "the graph page draws the canvas of the publication" do
    get public_note_graph_path("start-#{START}")

    assert_response :success
    assert_select "main [data-controller=graph-view][data-graph-view-url-value=?]", public_note_graph_path("start-#{START}", format: :json)
    assert_select "header a[href=?]", public_note_graph_path("start-#{START}")
  end

  test "the graph json links each note to its public address" do
    get public_note_graph_path("start-#{START}", format: :json)

    assert_response :success
    assert_equal [ [ SETUP, public_note_path("setup-#{SETUP}") ], [ START, public_note_path("start-#{START}") ] ],
                 response.parsed_body["nodes"].map { [ it["id"], it["url"] ] }
  end

  test "the graph of a private note answers 404 without a page" do
    get public_note_graph_path("secret-#{FakeMarkuppPublications.public_id_for("secret")}", format: :json)

    assert_response :not_found
    assert_empty response.body
  end
end
