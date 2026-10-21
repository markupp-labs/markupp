require "application_system_test_case"

# Filtros do grafo cheio além do painel (graph_view_test cobre as teclas de cada um): o vazio próprio quando eles escondem
# todas as notas, com o botão e a tecla de limpar.
class GraphFiltersTest < ApplicationSystemTestCase
  ROADMAP = { id: "roadmap", title: "roadmap", path: "roadmap.md", degree: 0, pagerank: 0.3, community: 2, unresolved: false }.freeze
  NOTES = [ { id: "plan", path: "work/plan.md", content: "# plan\n" }, { id: "roadmap", path: "roadmap.md", content: "# roadmap\n" } ].freeze
  GRAPH = { "vault-1" => { nodes: [ ROADMAP ], links: [] }, "vault-1?orphans=false" => { nodes: [], links: [] } }.freeze

  setup do
    use_fake_api(notes: NOTES, graph: { graph: GRAPH })
    sign_in_through_form
  end

  test "filters that hide every note say so instead of the empty vault, and x clears them" do
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"

    send_keys "o"
    assert_selector "main [data-graph-nodes='0']"
    assert_selector "main", text: "No notes match these filters"
    assert_no_selector "main", text: "No notes to draw yet"
    send_keys "x"

    assert_selector "main [data-graph-nodes='1']"
    assert_selector "#graph_filters input[name=orphans]:checked", visible: :all
    assert_equal({ "note" => "plan" }, page_query)
  end

  test "the clear filters button draws every note again and gives the focus back to the canvas" do
    visit graph_path(note: "plan", orphans: "false")
    assert_selector "main [data-graph-nodes='0']"

    assert_button "Clear filters"
    click_on "Clear filters"

    assert_selector "main [data-graph-nodes='1']"
    assert_selector "main canvas[data-graph-canvas]:focus"
    assert_equal "", find("#graph_filters .graph-filters-count").text
  end

  private
    def page_query = Rack::Utils.parse_query(URI(current_url).query)
end
