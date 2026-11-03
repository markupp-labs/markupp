require "test_helper"

# Grafo do cofre publicado, só leitura e sem conta: o mesmo json do canvas do painel, com o link público de cada nota.
class PublicGraphsControllerTest < ActionDispatch::IntegrationTest
  NOTES = [ { id: "start", path: "start.md", content: "" }, { id: "setup", path: "guides/setup.md", content: "" } ].freeze
  NODES = [
    { id: "start", title: "start", path: "start.md", degree: 1, pagerank: 0.6, community: 2, unresolved: false },
    { id: "setup", title: "setup", path: "guides/setup.md", degree: 1, pagerank: 0.4, community: nil, unresolved: false },
    { id: nil, title: "Ghost", path: nil, degree: nil, pagerank: nil, community: nil, unresolved: true }
  ].freeze
  GRAPH = { graph: { "vault-1" => { nodes: NODES, links: [ { source: 0, target: 1, type: "explicit", predicate: nil } ] } } }.freeze
  PUBLICATIONS = [ { vault_id: "vault-1", slug: "docs", title: "Markupp docs" } ].freeze

  setup do
    @api = use_fake_api(notes: NOTES, graph: GRAPH, publications: PUBLICATIONS)
  end

  test "the graph comes ready to draw, with the public link of each note" do
    get public_vault_graph_path("docs", format: :json)

    assert_response :success
    assert_equal [ [ "start", public_vault_path("docs", path: "start") ], [ "setup", public_vault_path("docs", path: "guides/setup") ], [ "Ghost", nil ] ],
                 Array(response.parsed_body["nodes"]).map { it.values_at("title", "url") }
    assert_equal [ [ 0, 1, false ] ], Array(response.parsed_body["links"]).map { it.values_at("source", "target", "inferred") }
  end

  test "a vault that is not published answers 404 without a body" do
    get public_vault_graph_path("private", format: :json)

    assert_response :not_found
    assert_empty response.body
  end

  test "a reader over the limit of the API gets 429" do
    @api.limit_public_reads!

    get public_vault_graph_path("docs", format: :json)

    assert_response :too_many_requests
  end
end
