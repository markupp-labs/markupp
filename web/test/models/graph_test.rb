require "test_helper"

class GraphTest < ActiveSupport::TestCase
  VAULT_GRAPH = {
    nodes: [
      { id: "1", title: "plan", path: "plan.md", degree: 1, pagerank: 0.6, community: 7, unresolved: false },
      { id: nil, title: "Someday", path: nil, degree: nil, pagerank: nil, community: nil, unresolved: true }
    ],
    links: [ { source: 0, target: 1, type: "typed", predicate: "next" } ]
  }.freeze

  setup do
    @store = CacheWithoutExpiry.new
    @api = FakeMarkuppApi.new(graph: { graph: { "vault-1" => VAULT_GRAPH, "1" => VAULT_GRAPH } })
  end

  test "of_vault builds the nodes and the links as indexes into the nodes" do
    graph = Graph.of_vault(@api, "vault-1", store: @store)

    assert_equal [ "1", "plan", "plan.md", 1, 0.6, 7, false ], graph.nodes.first&.to_h&.values
    assert_equal [ nil, "Someday", true ], graph.nodes.second&.then { [ it.id, it.title, it.unresolved ] }
    assert_equal [ Graph::Link.new(source: 0, target: 1, type: "typed", predicate: "next") ], graph.links
  end

  test "of_vault passes the filters on" do
    Graph.of_vault(@api, "vault-1", store: @store, orphans: false, unresolved: false, inferred: false, community: 7)

    assert_equal [ { "community" => "7", "inferred" => "false", "orphans" => "false", "unresolved" => "false" } ], @api.graph_queries
  end

  test "around_note builds the local graph of the note with the depth" do
    graph = Graph.around_note(@api, "1", store: @store, depth: 2)

    assert_equal 2, graph.nodes.size
    assert_equal [ { "depth" => "2" } ], @api.graph_queries
  end

  test "revalidates the stored graph by its ETag and keeps it when the API answers 304" do
    @api.change_graph!("vault-1", VAULT_GRAPH, etag: 'W/"1"')
    Graph.of_vault(@api, "vault-1", store: @store)

    graph = Graph.of_vault(@api, "vault-1", store: @store)

    assert_equal [ nil, 'W/"1"' ], @api.received_etags
    assert_equal 1, @api.graph_queries.size
    assert_equal 2, graph.nodes.size
  end

  test "replaces the stored graph when its version changes" do
    @api.change_graph!("vault-1", VAULT_GRAPH, etag: 'W/"1"')
    Graph.of_vault(@api, "vault-1", store: @store)
    @api.change_graph!("vault-1", { nodes: [], links: [] }, etag: 'W/"2"')

    assert_empty Graph.of_vault(@api, "vault-1", store: @store).nodes
    assert_empty Graph.of_vault(@api, "vault-1", store: @store).nodes
    assert_equal [ nil, 'W/"1"', 'W/"2"' ], @api.received_etags
  end

  test "stores nothing while the graph has no ETag yet" do
    2.times { Graph.of_vault(@api, "vault-1", store: @store) }

    assert_equal [ nil, nil ], @api.received_etags
  end
end
