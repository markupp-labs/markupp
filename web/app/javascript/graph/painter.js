// Desenho de um quadro do grafo no canvas, sem estado: ligações finas na cor muted, que passa de 3:1 nos dois palcos e
// nos dois temas (a inferida tracejada; a line, que quase some, fica para as apagadas), nós no matiz da comunidade com o raio do PageRank, o título não resolvido só em contorno, a nota aberta com o anel no accent e a nota
// com o foco do teclado com o anel no ink. Com um nó em destaque (hover ou foco), ele e as vizinhas ficam e o resto apaga.
// Os nomes aparecem nos mais centrais, no destaque e nas vizinhas, e em todos a partir de LABEL_ZOOM. Traço e fonte
// dividem pela escala: ficam do mesmo tamanho na tela em qualquer zoom.
const LABEL_ZOOM = 1.6
const FADED = 0.3
const FONT_PX = 11
const DASH_PX = 3
const LABEL_GAP_PX = 3
// Anéis em px da tela: a distância da borda do nó ao meio do traço e a largura do traço.
const CURRENT_RING = { gap: 3, width: 2 }
const FOCUS_RING = { gap: 6, width: 1.5 }

export function paintGraph(context, { nodes, links, colors, camera, ratio, emphasis }) {
  context.setTransform(ratio, 0, 0, ratio, 0, 0)
  context.clearRect(0, 0, camera.width, camera.height)
  camera.apply(context, ratio)
  const lit = litNodes(emphasis)
  paintLinks(context, links, colors, camera.k, lit)
  nodes.forEach((node) => paintNode(context, node, colors, camera.k, lit))
  paintLabels(context, nodes, colors, camera, emphasis, lit)
  context.globalAlpha = 1
}

// O nó em destaque e as vizinhas, ou null sem destaque (nada apaga).
function litNodes({ active }) {
  return active ? new Set([ active, ...active.neighbors ]) : null
}

function paintLinks(context, links, colors, k, lit) {
  for (const inferred of [ false, true ]) {
    context.setLineDash(inferred ? [ DASH_PX / k, DASH_PX / k ] : [])
    strokeLinks(context, links.filter((link) => link.inferred == inferred), lit, { color: lit ? colors.line : colors.muted, width: 1 / k })
    if (lit) strokeLinks(context, links.filter((link) => link.inferred == inferred && lit.has(link.source) && lit.has(link.target)), null,
                         { color: colors.accent, width: 1.5 / k })
  }
  context.setLineDash([])
}

function strokeLinks(context, links, lit, { color, width }) {
  context.globalAlpha = lit ? FADED : 1
  context.strokeStyle = color
  context.lineWidth = width
  context.beginPath()
  for (const { source, target } of links) {
    context.moveTo(source.x, source.y)
    context.lineTo(target.x, target.y)
  }
  context.stroke()
}

function paintNode(context, node, colors, k, lit) {
  context.globalAlpha = lit && !lit.has(node) ? FADED : 1
  context.beginPath()
  context.arc(node.x, node.y, node.radius, 0, 2 * Math.PI)
  if (node.unresolved) {
    context.lineWidth = 1 / k
    context.strokeStyle = colors.muted
    context.stroke()
  } else {
    context.fillStyle = node.hue == null ? colors.muted : colors.communities[node.hue]
    context.fill()
  }
  if (node.current) ring(context, node, colors.accent, CURRENT_RING.gap / k, CURRENT_RING.width / k)
  if (node.focused) ring(context, node, colors.ink, FOCUS_RING.gap / k, FOCUS_RING.width / k)
}

function ring(context, node, color, gap, width) {
  context.beginPath()
  context.arc(node.x, node.y, node.radius + gap, 0, 2 * Math.PI)
  context.lineWidth = width
  context.strokeStyle = color
  context.stroke()
}

function paintLabels(context, nodes, colors, camera, emphasis, lit) {
  const k = camera.k
  context.font = `${FONT_PX / k}px "Fira Code", monospace`
  context.textAlign = "center"
  context.textBaseline = "top"
  context.lineJoin = "round"
  context.lineWidth = 3 / k
  context.strokeStyle = colors.halo
  for (const node of nodes) {
    if (!labelled(node, k, emphasis, lit)) continue

    context.globalAlpha = 1
    context.fillStyle = node == emphasis.active || node.focused ? colors.ink : colors.muted
    const y = node.y + labelOffset(node, k)
    context.strokeText(node.title, node.x, y)
    context.fillText(node.title, node.x, y)
  }
}

// Do centro do nó ao topo do nome: passa da borda do anel mais de fora (o do foco, senão o da nota aberta) e deixa LABEL_GAP_PX.
function labelOffset(node, k) {
  const ring = node.focused ? FOCUS_RING : node.current ? CURRENT_RING : null
  const outer = ring ? (ring.gap + ring.width / 2) / k : 0
  return node.radius + outer + LABEL_GAP_PX / k
}

function labelled(node, k, emphasis, lit) {
  if (node.focused || node == emphasis.active) return true
  if (lit) return lit.has(node)
  return node.prominent || node.current || k >= LABEL_ZOOM
}
