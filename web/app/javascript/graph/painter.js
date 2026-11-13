// Desenho de um quadro do grafo no canvas, sem estado: cada método de ligação (origin) na sua cor e no seu traço, a
// manual fina e cheia no muted e as do markupp mais grossas, todas acima de 3:1 nos dois palcos e nos dois temas; a line, que quase some, fica para as apagadas. Nós no matiz da comunidade com
// o raio do PageRank, o título não resolvido só em contorno, a nota aberta com o anel no accent e a nota
// com o foco do teclado com o anel no ink. Com um nó em destaque (hover ou foco), ele e as vizinhas ficam e o resto apaga;
// com uma ligação em destaque (hover), ficam as duas pontas e as vizinhas de cada uma.
// Os nomes aparecem nos mais centrais, no destaque e nas vizinhas, e em todos a partir de LABEL_ZOOM. Traço e fonte
// dividem pela escala: ficam do mesmo tamanho na tela em qualquer zoom.
const LABEL_ZOOM = 1.6
const FADED = 0.3
const FONT_PX = 11
// Traço de cada método de ligação em px da tela, que se lê mesmo sem a cor: a manual fina e cheia, e as que o markupp
// faz mais grossas, cada uma num desenho (a inferida em traços longos, a menção em pontos redondos). Sem origin
// conhecido, a automática fica nos pontos.
const LINK_STYLES = {
  manual: { dash: [], width: 1 },
  inferred: { dash: [ 8, 4 ], width: 1.75 },
  mentioned: { dash: [ 1.5, 4 ], width: 1.75 }
}
const LIT_EXTRA_WIDTH = 0.5
const LABEL_GAP_PX = 3
// Anéis em px da tela: a distância da borda do nó ao meio do traço e a largura do traço.
const CURRENT_RING = { gap: 3, width: 2 }
const FOCUS_RING = { gap: 6, width: 1.5 }
// Zoom a partir do qual a bolinha para de crescer na tela: o enquadramento aproxima o grafo para ocupar o canvas, e
// sem o teto as bolinhas ficariam enormes num cofre pequeno.
const NODE_GROWTH_ZOOM = 2

// Raio do nó no mundo: o do PageRank até NODE_GROWTH_ZOOM, encolhendo depois para manter o tamanho na tela.
export function drawnRadius(node, k) {
  return node.radius * Math.min(1, NODE_GROWTH_ZOOM / k)
}

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

// O nó em destaque e as vizinhas, as pontas da ligação em destaque e as vizinhas delas, ou null sem destaque (nada apaga).
function litNodes({ active }) {
  if (!active) return null
  return new Set(endsOf(active).flatMap((node) => [ node, ...node.neighbors ]))
}

function endsOf(active) {
  return active.source ? [ active.source, active.target ] : [ active ]
}

function emphasized(node, { active }) {
  return active != null && endsOf(active).includes(node)
}

function paintLinks(context, links, colors, k, lit) {
  for (const origin of new Set(links.map(originOf))) {
    const { dash, width } = LINK_STYLES[origin] ?? LINK_STYLES.mentioned
    context.setLineDash(dash.map((px) => px / k))
    context.lineCap = origin == "manual" ? "butt" : "round"
    const drawn = links.filter((link) => originOf(link) == origin)
    strokeLinks(context, drawn, lit, { color: lit ? colors.line : restColor(origin, colors), width: width / k })
    if (lit) strokeLinks(context, drawn.filter((link) => lit.has(link.source) && lit.has(link.target)), null, { color: litColor(origin, colors), width: (width + LIT_EXTRA_WIDTH) / k })
  }
  context.setLineDash([])
  context.lineCap = "butt"
}

// O método da ligação; sem origin, pelo tipo.
function originOf({ origin, inferred, automatic }) {
  return origin ?? (automatic ? "automatic" : inferred ? "inferred" : "manual")
}

// A manual é neutra; cada método do markupp tem a sua cor, e o que não tem cor própria fica no accent.
function restColor(origin, colors) {
  return origin == "manual" ? colors.muted : colors.links?.[origin] ?? colors.accent
}

function litColor(origin, colors) {
  return origin == "manual" ? colors.ink : restColor(origin, colors)
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
  context.arc(node.x, node.y, drawnRadius(node, k), 0, 2 * Math.PI)
  if (node.unresolved) {
    context.lineWidth = 1 / k
    context.strokeStyle = colors.muted
    context.stroke()
  } else {
    context.fillStyle = node.hue == null ? colors.muted : colors.communities[node.hue]
    context.fill()
  }
  if (node.current) ring(context, node, drawnRadius(node, k), colors.accent, CURRENT_RING.gap / k, CURRENT_RING.width / k)
  if (node.focused) ring(context, node, drawnRadius(node, k), colors.ink, FOCUS_RING.gap / k, FOCUS_RING.width / k)
}

function ring(context, node, radius, color, gap, width) {
  context.beginPath()
  context.arc(node.x, node.y, radius + gap, 0, 2 * Math.PI)
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
    context.fillStyle = emphasized(node, emphasis) || node.focused ? colors.ink : colors.muted
    const y = node.y + labelOffset(node, k)
    context.strokeText(node.title, node.x, y)
    context.fillText(node.title, node.x, y)
  }
}

// Do centro do nó ao topo do nome: passa da borda do anel mais de fora (o do foco, senão o da nota aberta) e deixa LABEL_GAP_PX.
function labelOffset(node, k) {
  const ring = node.focused ? FOCUS_RING : node.current ? CURRENT_RING : null
  const outer = ring ? (ring.gap + ring.width / 2) / k : 0
  return drawnRadius(node, k) + outer + LABEL_GAP_PX / k
}

function labelled(node, k, emphasis, lit) {
  if (node.focused || emphasized(node, emphasis)) return true
  if (lit) return lit.has(node)
  return node.prominent || node.current || k >= LABEL_ZOOM
}
