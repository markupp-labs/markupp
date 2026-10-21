require "application_system_test_case"

# Grafo de força em canvas, como o graph view do Obsidian: a visão Graph da barra lateral desenha a nota aberta e as
# vizinhas, e Alt+G abre o cofre inteiro em tela cheia, de onde o Esc volta. O módulo do grafo só baixa quando um dos
# dois abre. O canvas diz no data-graph-nodes e no data-graph-links o que desenhou; j e k andam pelas notas e Enter abre.
class GraphViewTest < ApplicationSystemTestCase
  PLAN = { id: "plan", title: "plan", path: "work/plan.md", degree: 2, pagerank: 0.5, community: 1, unresolved: false }.freeze
  ROADMAP = { id: "roadmap", title: "roadmap", path: "roadmap.md", degree: 1, pagerank: 0.3, community: 2, unresolved: false }.freeze
  GHOST = { id: nil, title: "Ghost", path: nil, degree: nil, pagerank: nil, community: nil, unresolved: true }.freeze
  NOTES = [ PLAN, ROADMAP ].map { { id: it[:id], path: it[:path], content: "# #{it[:title]}\n" } }.freeze
  LOCAL = { nodes: [ PLAN, ROADMAP, GHOST ], links: [ { source: 0, target: 1, type: "explicit" }, { source: 0, target: 2, type: "explicit" },
                                                     { source: 1, target: 0, type: "inferred" } ] }.freeze
  VAULT = { nodes: [ ROADMAP ], links: [] }.freeze
  COMMUNITIES = [ { id: 7, size: 2, hub: { id: "plan", title: "plan", path: "work/plan.md" } } ].freeze

  setup do
    use_fake_api(notes: NOTES, graph: { graph: { "plan" => LOCAL, "vault-1" => VAULT, "vault-1?orphans=false" => { nodes: [], links: [] } },
                                         communities: { "vault-1" => COMMUNITIES } })
    sign_in_through_form
    visit note_path("plan")
    assert_field "Path", with: "/work/plan"
  end

  test "the graph module downloads only when the graph view opens, which draws the local graph and focuses it" do
    assert_not graph_module_requested?, "the graph module must not download before the graph opens"

    send_keys [ :control, :shift, "g" ]

    assert_selector "#sidebar_view_graph canvas[data-graph-canvas]:focus"
    assert_selector "#sidebar_view_graph [data-graph-nodes='3'][data-graph-links='3']"
    assert graph_module_requested?
    assert_selector "#sidebar_view_graph [role=status]", text: "3 nodes  3 links"
  end

  test "j walks the notes from the most central, enter opens one and the view follows the open note" do
    send_keys [ :control, :shift, "g" ]
    assert_selector "#sidebar_view_graph [data-graph-nodes='3']"

    send_keys "j"
    assert_selector "#sidebar_view_graph [data-graph-focus=plan]"
    send_keys "j"
    assert_selector "#sidebar_view_graph [data-graph-focus=roadmap]"
    send_keys :enter

    assert_field "Path", with: "/roadmap"
  end

  test "a graph taken out of the page and put back draws again and answers the keys" do
    send_keys [ :control, :shift, "g" ]
    assert_selector "#sidebar_view_graph [data-graph-nodes='3']"

    evaluate_async_script(<<~JS)
      const done = arguments[arguments.length - 1]
      const graph = document.querySelector("#sidebar_view_graph [data-controller~=graph-view]")
      const [ parent, next ] = [ graph.parentNode, graph.nextSibling ]
      graph.remove()
      setTimeout(() => { parent.insertBefore(graph, next); done() }, 50)
    JS
    find("#sidebar_view_graph canvas[data-graph-canvas]").execute_script("this.focus()")
    send_keys "j"

    assert_selector "#sidebar_view_graph [data-graph-focus=plan]"
  end

  test "the depth buttons redraw the graph two hops away and back" do
    send_keys [ :control, :shift, "g" ]
    assert_selector "#sidebar_view_graph [data-graph-nodes='3']"

    click_on "2"

    assert_selector "button[aria-pressed=true]", text: "2"
    assert_selector "button[aria-pressed=false]", text: "1"
    assert_selector "#sidebar_view_graph [data-graph-depth='2'][data-graph-nodes='3']"
  end

  test "alt+g opens the full graph of the vault, a click on a note opens it and esc goes back where it came from" do
    send_keys [ :alt, "g" ]

    assert_selector "main h1", text: "Graph"
    assert_selector "main canvas[data-graph-canvas]:focus"
    assert_selector "main [data-graph-nodes='1'][data-graph-links='0']"
    send_keys :escape
    assert_field "Path", with: "/work/plan"

    send_keys [ :alt, "g" ]
    canvas = find("main canvas[data-graph-canvas]")
    assert_selector "main [data-graph-nodes='1']"
    canvas.click
    assert_field "Path", with: "/roadmap"
  end

  test "plus and minus zoom the full graph and zero fits it again" do
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"
    find("main canvas[data-graph-canvas]").execute_script("this.focus()")
    fitted = find("main [data-graph-zoom]")["data-graph-zoom"]

    send_keys "="
    assert_operator find("main [data-graph-zoom]")["data-graph-zoom"].to_f, :>, fitted.to_f
    send_keys "0"
    assert_selector "main [data-graph-zoom='#{fitted}']"
    send_keys "-"

    assert_operator find("main [data-graph-zoom]")["data-graph-zoom"].to_f, :<, fitted.to_f
  end

  test "a full graph of more than two notes fills the canvas, past the zoom that keeps one or two notes small" do
    star = (0..10).map { { id: "n#{it}", title: "n#{it}", path: "n#{it}.md", degree: 1, pagerank: 0.1, community: 1, unresolved: false } }
    use_fake_api(notes: NOTES, graph: { graph: { "vault-1" => { nodes: star, links: (1..10).map { { source: 0, target: it, type: "explicit" } } } } })
    sign_in_through_form
    visit graph_path(note: "plan")

    assert_selector "main [data-graph-nodes='11'][data-graph-running=false][data-graph-zoom]"
    assert_operator find("main [data-graph-zoom]")["data-graph-zoom"].to_f, :>, 2
  end

  test "o hides the orphan notes of the full graph, the page address keeps the filter and a reload draws it again" do
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"

    send_keys "o"
    assert_selector "main [data-graph-nodes='0']"
    assert_equal({ "note" => "plan", "orphans" => "false" }, page_query)
    refresh
    assert_selector "main [data-graph-nodes='0']"
    assert_selector "#graph_filters input[name=orphans]:not(:checked)", visible: :all
    find("main canvas[data-graph-canvas]").execute_script("this.focus()")
    send_keys "o"

    assert_selector "main [data-graph-nodes='1']"
    assert_equal({ "note" => "plan" }, page_query)
  end

  test "c and shift+c step through the communities of the full graph" do
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"

    send_keys "c"
    assert_equal({ "note" => "plan", "community" => "7" }, page_query)
    assert_selector "#graph_filters select[name=community] option[value='7']:checked", text: "plan (2)", visible: :all
    send_keys [ :shift, "c" ]

    assert_equal({ "note" => "plan" }, page_query)
  end

  test "f opens the filters over the canvas, alt+k reaches them, esc closes them and alt+j goes back to the canvas" do
    visit graph_path(note: "plan")
    assert_selector "main canvas[data-graph-canvas]:focus"

    send_keys "f"
    assert_selector "#graph_filters[open] input[name=orphans]"
    assert_selector "main canvas[data-graph-canvas]:focus"
    send_keys [ :alt, "k" ]
    assert_selector "#graph_filters summary:focus"
    send_keys :escape
    assert_selector "#graph_filters:not([open]) summary:focus"
    assert_selector "main h1", text: "Graph"
    send_keys [ :alt, "j" ]

    assert_selector "main canvas[data-graph-canvas]:focus"
  end

  test "on a phone the full graph fits the screen, status line included, without scrolling the page" do
    page.current_window.resize_to(390, 844)
    visit graph_path(note: "plan")
    assert_selector "main [data-graph-nodes='1']"

    assert_equal evaluate_script("window.innerHeight"), evaluate_script("document.documentElement.scrollHeight")
    assert_operator edge(find("main [role=status]"), "bottom"), :<=, evaluate_script("window.innerHeight")
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "with reduced motion the layout is worked out at once and drawn still" do
    send_keys [ :control, :shift, "g" ]

    assert_selector "#sidebar_view_graph [data-graph-nodes='3'][data-graph-running=false]"
  end

  test "with motion the simulation settles and stops asking for frames" do
    emulate_motion_preference("no-preference")
    send_keys [ :control, :shift, "g" ]

    assert_selector "#sidebar_view_graph [data-graph-nodes='3']"
    assert_selector "#sidebar_view_graph [data-graph-running=false]", wait: 10
  end

  test "alt+h leaves the graph for the activity bar and alt+l comes back to the canvas" do
    send_keys [ :control, :shift, "g" ]
    assert_selector "#sidebar_view_graph canvas[data-graph-canvas]:focus"

    send_keys [ :alt, "h" ]
    assert_selector ".activity-bar-side [role=tab][aria-label=Graph]:focus"
    send_keys [ :alt, "l" ]

    assert_selector "#sidebar_view_graph canvas[data-graph-canvas]:focus"
  end

  test "the palette opens the full graph" do
    send_keys [ :control, :shift, "p" ]
    assert_selector "dialog#command_palette[open] input:focus"
    send_keys "full graph", :enter

    assert_selector "main h1", text: "Graph"
  end

  private
    def page_query = Rack::Utils.parse_query(URI(current_url).query)

    def graph_module_requested?
      evaluate_script("performance.getEntriesByType('resource').some((entry) => /d3-force|graph\\/scene/.test(entry.name))")
    end
end
