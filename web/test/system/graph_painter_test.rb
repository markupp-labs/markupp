require "application_system_test_case"

# Desenho de um quadro do grafo (graph/painter.js) contra um contexto de canvas falso que anota cada traço e cada nome:
# o nome fica abaixo do anel mais de fora do nó, e as ligações em repouso usam a cor muted, que contrasta 3:1 ou mais
# com o palco; a line fica só para as apagadas.
class GraphPainterTest < ApplicationSystemTestCase
  setup do
    use_fake_api(notes: [])
    sign_in_through_form
    visit root_path
  end

  test "the name sits below the focus ring and below the ring of the open note" do
    labels = painted(nodes: "[ node('focused', { focused: true }), node('current', { current: true }), node('plain', {}) ]")["labels"]

    assert_operator labels["focused"], :>=, 5 + 6 + 0.75 + 3
    assert_operator labels["current"], :>=, 5 + 4 + 3
    assert_in_delta 5 + 3, labels["plain"]
  end

  test "links at rest are stroked in the muted ink and the faded ones in line" do
    rest = painted(nodes: "[ node('a', {}), node('b', {}) ]")["linkStrokes"]
    lit = painted(nodes: "[ node('a', {}), node('b', {}) ]", active: 0)["linkStrokes"]

    assert_equal %w[muted muted], rest
    assert_equal %w[line accent line accent], lit
  end

  private
    # Pinta com o FakeCanvasContext e devolve o y de cada nome (o nó está em y 0, raio 5, zoom 1) e a cor de cada
    # traço de ligação, na ordem.
    def painted(nodes:, active: nil)
      evaluate_async_script(<<~JS)
        const done = arguments[arguments.length - 1]
        import("graph/painter").then(({ paintGraph }) => {
          class FakeCanvasContext {
            labels = {}
            strokes = []
            setTransform() {} clearRect() {} setLineDash() {} beginPath() {} moveTo() {} lineTo() {} arc() {} fill() {}
            stroke() { this.strokes.push(this.strokeStyle) }
            strokeText() {}
            fillText(text, x, y) { this.labels[text] = y }
          }
          const node = (title, flags) => ({ title, x: 0, y: 0, radius: 5, hue: 0, neighbors: [], prominent: true, ...flags })
          const nodes = #{nodes}
          const links = [ { source: nodes[0], target: nodes[1], inferred: false } ]
          const active = #{active.nil? ? "null" : "nodes[#{active}]"}
          if (active) active.neighbors = [ nodes[1] ]
          const context = new FakeCanvasContext()
          const colors = { line: "line", ink: "ink", muted: "muted", accent: "accent", halo: "halo", communities: [ "c0" ] }
          const camera = { width: 100, height: 100, k: 1, apply() {} }
          paintGraph(context, { nodes, links, colors, camera, ratio: 1, emphasis: { active } })
          done({ labels: context.labels, linkStrokes: context.strokes.slice(0, active ? 4 : 2) })
        })
      JS
    end
end
