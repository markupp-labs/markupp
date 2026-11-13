require "application_system_test_case"

# Desenho de um quadro do grafo (graph/painter.js) contra um contexto de canvas falso que anota cada traço e cada nome:
# o nome fica abaixo do anel mais de fora do nó, cada método de ligação tem a sua cor e o seu traço, e a line fica só
# para as apagadas.
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

  # A cor diz o método: a manual fica no muted (no ink quando acesa) e cada método do markupp na cor dele, mais grosso
  # quando aceso.
  test "each link method has its own color, the manual one muted, at rest and lit" do
    colors = lambda do |active|
      evaluate_async_script(<<~JS)
        const done = arguments[arguments.length - 1]
        import("graph/painter").then(({ paintGraph }) => {
          const byDash = {}
          let dash = ""
          const context = new Proxy({}, { get: (target, key) => ({ setLineDash: (d) => { dash = d.join(",") }, stroke: () => { (byDash[dash] ||= []).push(target.strokeStyle) } })[key] ?? target[key] ?? (() => {}),
                                           set: (target, key, value) => { target[key] = value; return true } })
          const node = (title) => ({ title, x: 0, y: 0, radius: 5, hue: 0, neighbors: [] })
          const [ a, b ] = [ node("a"), node("b") ]
          a.neighbors = [ b ]; b.neighbors = [ a ]
          const links = [ { source: a, target: b, origin: "manual" }, { source: a, target: b, origin: "inferred" }, { source: a, target: b, origin: "mentioned" } ]
          const colors = { line: "line", ink: "ink", muted: "muted", accent: "accent", halo: "halo", communities: [ "c0" ],
                           links: { inferred: "yellow", mentioned: "blue" } }
          paintGraph(context, { nodes: [ a, b ], links, colors, camera: { width: 100, height: 100, k: 1, apply() {} }, ratio: 1, emphasis: { active: #{active} } })
          done(byDash)
        }).catch((error) => done({ error: String(error) }))
      JS
    end

    assert_equal({ "" => [ "muted" ], "8,4" => [ "yellow" ], "1.5,4" => [ "blue" ] }, colors.call("null"))
    assert_equal({ "" => %w[line ink], "8,4" => %w[line yellow], "1.5,4" => %w[line blue] }, colors.call("a"))
  end

  # O enquadramento aproxima o grafo para ocupar o canvas; a bolinha para de crescer na tela a partir do zoom 2.
  test "a node stops growing on screen past zoom 2" do
    at_two = painted(nodes: "[ node('a', {}), node('b', {}) ]", zoom: 2)["arcs"].first * 2
    at_four = painted(nodes: "[ node('a', {}), node('b', {}) ]", zoom: 4)["arcs"].first * 4

    assert_in_delta at_two, at_four
  end

  # Passar sobre uma ligação acende as duas pontas e as vizinhas delas, como o destaque de um nó, com o nome das pontas no ink.
  test "a link in emphasis lights both ends and their neighbors and names the ends in ink" do
    painted = evaluate_async_script(<<~JS)
      const done = arguments[arguments.length - 1]
      import("graph/painter").then(({ paintGraph }) => {
        const fills = {}
        const context = new Proxy({}, { get: (target, key) => key == "fillText" ? (text) => { fills[text] = { color: target.fillStyle, alpha: target.globalAlpha } } : target[key] ?? (() => {}),
                                         set: (target, key, value) => { target[key] = value; return true } })
        const node = (title) => ({ title, x: 0, y: 0, radius: 5, hue: 0, neighbors: [] })
        const [ a, b, c, far ] = [ node("a"), node("b"), node("c"), node("far") ]
        a.neighbors = [ b ]; b.neighbors = [ a, c ]; c.neighbors = [ b ]
        const links = [ { source: a, target: b }, { source: b, target: c } ]
        const colors = { line: "line", ink: "ink", muted: "muted", accent: "accent", halo: "halo", communities: [ "c0" ] }
        paintGraph(context, { nodes: [ a, b, c, far ], links, colors, camera: { width: 100, height: 100, k: 1, apply() {} }, ratio: 1, emphasis: { active: links[0] } })
        done(fills)
      }).catch((error) => done({ error: String(error) }))
    JS

    assert_equal %w[a b c], painted.keys.sort, "only the lit notes are named"
    assert_equal %w[ink ink muted], %w[a b c].map { painted[it]["color"] }
  end

  private
    # Pinta com o FakeCanvasContext e devolve o y de cada nome (o nó está em y 0, raio 5), o raio de cada arco e a cor de cada
    # traço de ligação, na ordem.
    def painted(nodes:, active: nil, zoom: 1)
      evaluate_async_script(<<~JS)
        const done = arguments[arguments.length - 1]
        import("graph/painter").then(({ paintGraph }) => {
          class FakeCanvasContext {
            labels = {}
            strokes = []
            arcs = []
            dashes = []
            setTransform() {} clearRect() {} beginPath() {}
            setLineDash(dash) { this.dash = dash.join(",") } moveTo() {} lineTo() {} fill() {}
            arc(x, y, radius) { this.arcs.push(radius) }
            stroke() { this.strokes.push(this.strokeStyle); this.dashes.push(this.dash) }
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
          const camera = { width: 100, height: 100, k: #{zoom}, apply() {} }
          paintGraph(context, { nodes, links, colors, camera, ratio: 1, emphasis: { active } })
          done({ labels: context.labels, arcs: context.arcs, linkStrokes: context.strokes.slice(0, active ? 4 : 2), linkDashes: context.dashes.slice(0, 3) })
        })
      JS
    end
end
