require "application_system_test_case"

# Filtros do grafo cheio além do painel (graph_view_test cobre as teclas de cada um): o vazio próprio quando eles escondem
# todas as notas, com o botão e a tecla de limpar, e o histórico: cada troca é um passo que o voltar desfaz.
class GraphFiltersTest < ApplicationSystemTestCase
  ROADMAP = { id: "roadmap", title: "roadmap", path: "roadmap.md", degree: 0, pagerank: 0.3, community: 2, unresolved: false }.freeze
  NOTES = [ { id: "plan", path: "work/plan.md", content: "# plan\n" }, { id: "roadmap", path: "roadmap.md", content: "# roadmap\n" } ].freeze
  GRAPH = { "vault-1" => { nodes: [ ROADMAP ], links: [] }, "vault-1?orphans=false" => { nodes: [], links: [] } }.freeze
  COMMUNITIES = [ { id: 7, size: 2, hub: { id: "plan", title: "plan", path: "work/plan.md" } } ].freeze

  setup do
    use_fake_api(notes: NOTES, graph: { graph: GRAPH, communities: { "vault-1" => COMMUNITIES } })
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
    assert_equal "", find("#graph_filters .graph-filters-count", visible: :all).text(:all)
  end

  test "back undoes one filter at a time and redraws without loading the page again" do
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"
    send_keys "o"
    assert_selector "main [data-graph-nodes='0']"
    send_keys "c"
    assert_equal({ "note" => "plan", "orphans" => "false", "community" => "7" }, page_query)
    execute_script("document.querySelector('#vault_graph').dataset.samePage = 'yes'")

    go_back
    assert_equal({ "note" => "plan", "orphans" => "false" }, page_query)
    assert_selector "#graph_filters select[name=community] option[value='']:checked", visible: :all
    assert_selector "#graph_filters .graph-filters-count", text: "[1]"
    go_back

    assert_equal({ "note" => "plan" }, page_query)
    assert_selector "#vault_graph[data-same-page=yes][data-graph-nodes='1']"
    assert_selector "#graph_filters input[name=orphans]:checked", visible: :all
  end

  test "back from a note opened by the filtered graph shows it filtered, and the next back undoes the filter" do
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"
    send_keys "i"
    assert_equal({ "note" => "plan", "inferred" => "false" }, page_query)
    send_keys "j", :enter
    assert_field "Path", with: "/roadmap"

    go_back
    assert_selector "main [data-graph-nodes='1']"
    assert_selector "#graph_filters input[name=inferred]:not(:checked)", visible: :all
    execute_script("document.querySelector('#vault_graph').dataset.samePage = 'yes'")
    go_back

    assert_equal({ "note" => "plan" }, page_query)
    assert_selector "#vault_graph[data-same-page=yes]"
    assert_selector "#graph_filters input[name=inferred]:checked", visible: :all
  end

  test "back, forward and back again from a note opened by the filtered graph shows the filtered graph" do
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"
    send_keys "i"
    send_keys "j", :enter
    assert_field "Path", with: "/roadmap"
    go_back
    assert_selector "main [data-graph-nodes='1']"
    go_forward
    assert_field "Path", with: "/roadmap"

    go_back
    assert_equal({ "note" => "plan", "inferred" => "false" }, page_query)
    assert_selector "main [data-graph-nodes='1']"
    assert_selector "#graph_filters input[name=inferred]:not(:checked)", visible: :all
  end

  test "back from the graph to the note and forward again shows the graph" do
    visit note_path("plan")
    send_keys [ :alt, "g" ]
    assert_selector "main [data-graph-nodes='1']"
    go_back
    assert_field "Path", with: "/work/plan"

    go_forward
    assert_selector "main [data-graph-nodes='1']"
  end

  private
    def page_query = Rack::Utils.parse_query(URI(current_url).query)
end
