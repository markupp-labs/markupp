require "test_helper"

# Grafo como o graph view do Obsidian: a tela cheia do cofre (/graph) e a visão Graph da barra lateral, com a nota aberta
# e as vizinhas. As páginas desenham o canvas; os nós e as ligações vêm em json, prontos para desenhar.
class GraphsControllerTest < ActionDispatch::IntegrationTest
  NOTES = [ { id: "plan", path: "work/plan.md", content: "" }, { id: "roadmap", path: "roadmap.md", content: "" } ].freeze
  NODES = [
    { id: "plan", title: "plan", path: "work/plan.md", degree: 2, pagerank: 0.6, community: 9, unresolved: false },
    { id: "roadmap", title: "roadmap", path: "roadmap.md", degree: 1, pagerank: 0.3, community: nil, unresolved: false },
    { id: nil, title: "Ghost", path: nil, degree: nil, pagerank: nil, community: nil, unresolved: true }
  ].freeze
  LINKS = [ { source: 0, target: 1, type: "explicit", predicate: nil }, { source: 1, target: 0, type: "inferred", predicate: nil },
            { source: 0, target: 2, type: "typed", predicate: "next" } ].freeze
  GRAPH = { graph: { "vault-1" => { nodes: NODES, links: LINKS }, "plan" => { nodes: NODES, links: LINKS } } }.freeze

  setup do
    @api = use_fake_api(notes: NOTES, graph: GRAPH)
    sign_in_as
  end

  test "the vault graph comes ready to draw: the note url, the community hue, the rank and the inferred links" do
    get graph_path(format: :json, vault: "vault-1", note: "roadmap")

    body = response.parsed_body
    assert_equal [ [ "plan", note_path("plan"), 1, 0.6, false, false ], [ "roadmap", note_path("roadmap"), nil, 0.3, false, true ],
                   [ "Ghost", nil, nil, nil, true, false ] ],
                 Array(body["nodes"]).map { it.values_at("title", "url", "hue", "rank", "unresolved", "current") }
    assert_equal [ [ 0, 1, false ], [ 1, 0, true ], [ 0, 2, false ] ], Array(body["links"]).map { it.values_at("source", "target", "inferred") }
    assert_equal 3, body["total"]
  end

  test "a vault bigger than the cap keeps the most central nodes and says how many there were" do
    nodes = (0..GraphDrawing::NODE_LIMIT).map { { id: "n#{it}", title: "n#{it}", path: "n#{it}.md", pagerank: it.to_f, unresolved: false } }
    @api = use_fake_api(notes: NOTES, graph: { graph: { "vault-1" => { nodes:, links: [] } } })
    sign_in_as

    get graph_path(format: :json, vault: "vault-1")

    body = response.parsed_body
    assert_equal GraphDrawing::NODE_LIMIT, Array(body["nodes"]).size
    assert_equal GraphDrawing::NODE_LIMIT + 1, body["total"]
    assert_not_includes Array(body["nodes"]).map { it["title"] }, "n0"
  end

  test "a server that does not answer the graph gets a bad gateway, without a page" do
    @api.cut_connection!

    get graph_path(format: :json, vault: "vault-1")

    assert_response :bad_gateway
  end

  test "the full graph page draws the canvas of the vault of the note it came from, with esc back to that note" do
    get graph_path(note: "roadmap")

    assert_response :success
    assert_select "body[data-area=notes]"
    assert_select "main [data-controller=graph-view][data-focus-region=graph][data-graph-view-url-value=?]",
                  graph_path(format: :json, vault: "vault-1", note: "roadmap") do
      assert_select "canvas[data-graph-canvas][tabindex='0'][role=application][aria-label]"
    end
    assert_select "main a[href=?][aria-keyshortcuts=Escape][data-controller=area-back]", note_path("roadmap")
    assert_select "main h1", text: "Graph"
    assert_select "nav#sidebar [role=treeitem]", text: /roadmap/
  end

  test "the full graph page without a note goes back to the start, and an empty tenant has nothing to draw" do
    get graph_path

    assert_select "[data-graph-view-url-value=?]", graph_path(format: :json, vault: "vault-1")
    assert_select "main a[href=?][aria-keyshortcuts=Escape]", root_path

    use_fake_api(notes: [])
    sign_in_as
    get graph_path

    assert_select "[data-controller=graph-view]", count: 0
    assert_select "main", text: /No notes to draw yet/
  end

  test "the local graph of a note reads one hop by default and two when asked, never more" do
    get note_graph_path("plan", format: :json)
    get note_graph_path("plan", format: :json, depth: 2)
    get note_graph_path("plan", format: :json, depth: 9)

    assert_equal %w[1 2 2], @api.graph_queries.map { it["depth"] }
    assert_equal [ true, false, false ], Array(response.parsed_body["nodes"]).map { it["current"] }
  end

  test "the graph view of an open note draws its local graph with the depth and the way to the full graph" do
    cookies[:sidebar_view] = "graph"

    get note_path("plan")

    assert_select "#sidebar_view_graph:not([hidden]) [data-controller=graph-view][data-graph-view-url-value=?]", note_graph_path("plan", format: :json) do
      assert_select "canvas[data-graph-canvas][tabindex='0']"
      assert_select "button[aria-pressed=true][data-graph-view-depth-param='1']", text: "1"
      assert_select "button[aria-pressed=false][data-graph-view-depth-param='2']", text: "2"
    end
    assert_select "#sidebar_view_graph a[href=?][aria-keyshortcuts='Alt+G'][title=?]", graph_path(note: "plan"), "Full graph (Alt+G)"
    assert_select "#sidebar_view_graph", text: /Open a note to see its graph/, count: 0
  end
end
