require "application_system_test_case"

# Recusar ligação pelo grafo: clicar numa ligação automática pergunta pelo diálogo do painel e, aceito, a ligação sai do
# desenho na hora e o status diz que foi recusada. Duas notas ligadas ficam enquadradas com o meio da ligação no centro
# do canvas, onde o clique cai.
class GraphLinkRefusalTest < ApplicationSystemTestCase
  PLAN = { id: "plan", title: "plan", path: "plan.md", degree: 1, pagerank: 0.5, community: nil, unresolved: false }.freeze
  ROADMAP = { id: "roadmap", title: "roadmap", path: "roadmap.md", degree: 1, pagerank: 0.5, community: nil, unresolved: false }.freeze
  NOTES = [ PLAN, ROADMAP ].map { { id: it[:id], path: it[:path], content: "" } }.freeze

  def draw_with(type)
    use_fake_api(notes: NOTES, graph: { graph: { "vault-1" => { nodes: [ PLAN, ROADMAP ], links: [ { source: 0, target: 1, type: } ] } } })
    sign_in_through_form
    visit graph_path
    assert_selector "main [data-graph-nodes='2'][data-graph-links='1']"
  end

  test "a click on an automatic link asks, refuses it and takes it out of the drawing" do
    draw_with("mentioned")

    accept_panel_confirm("Refuse the automatic link between plan and roadmap?", button: "Refuse link") { find("main canvas[data-graph-canvas]").click }

    assert_selector "main [data-graph-nodes='2'][data-graph-links='0']"
    assert_selector "main [role=status]", text: "Link refused."
  end

  # A mão aparece só onde o clique faz algo: na ligação automática, que se recusa.
  test "the pointer over an automatic link turns into a hand and over a written one does not" do
    draw_with("mentioned")
    find("main canvas[data-graph-canvas]").hover
    assert_selector "main canvas[data-graph-canvas][data-hovering]"

    draw_with("explicit")
    find("main canvas[data-graph-canvas]").hover
    assert_no_selector "main canvas[data-graph-canvas][data-hovering]"
  end

  # A legenda do grafo cheio mostra só os métodos desenhados.
  test "the legend shows only the methods in the drawing" do
    draw_with("mentioned")

    within("main .graph-legend") do
      assert_selector "li", count: 1
      assert_selector "li", text: "mention"
    end
  end

  test "a click on a written link asks nothing" do
    draw_with("explicit")

    find("main canvas[data-graph-canvas]").click

    assert_no_selector "dialog#confirm_dialog[open]"
    assert_selector "main [data-graph-links='1']"
  end
end
