import { createForceLayout } from "graph/force_layout"
import { Camera } from "graph/camera"
import { paintGraph } from "graph/painter"
import { attachPointerGestures } from "graph/pointer_gestures"
import { readThemeColors } from "graph/theme_colors"

// Cena do grafo num canvas (graph_view_controller.js a baixa sob demanda): os nós e as ligações do json, a simulação de
// força, a câmera e o desenho. O requestAnimationFrame só roda enquanto a simulação anda, um nó é arrastado ou a vista
// muda; assentada, a cena para e só redesenha quando algo pede. Fora da vista (setVisible false) a simulação pausa, e
// numa aba escondida o navegador já não chama o quadro. Com reduced-motion o layout assenta de uma vez, sem animar.
// Até o primeiro gesto na vista a câmera acompanha o layout, enquadrando os nós; o 0 volta a enquadrar.
const RADIUS_MIN = 2.5
const RADIUS_SPREAD = 4.5
const UNRESOLVED_RADIUS = 2.5
const PROMINENT_LABELS = 8
const HIT_SLOP_PX = 4
const ZOOM_STEP = 1.25

export function createGraphScene(canvas, graph, callbacks) {
  return new GraphScene(canvas, graph, callbacks)
}

class GraphScene {
  emphasis = { active: null }
  focusIndex = -1
  following = true
  visible = true

  constructor(canvas, graph, { reducedMotion, onState, onOpen }) {
    Object.assign(this, { canvas, reducedMotion, onState, onOpen, context: canvas.getContext("2d") })
    this.nodes = graph.nodes.map(nodeOf)
    this.links = graph.links.map(({ source, target, inferred }) => ({ source: this.nodes[source], target: this.nodes[target], inferred }))
    connectNeighbors(this.links)
    markProminent(this.nodes)
    this.walk = this.nodes.filter((node) => node.url).sort(byRank)
    this.layout = createForceLayout(this.nodes, this.links, (node) => node.radius)
    this.camera = new Camera()
    this.colors = readThemeColors(canvas.parentElement)
    this.detachGestures = attachPointerGestures(canvas, this)
    this.running = !reducedMotion
    if (reducedMotion) this.layout.settle()
    this.resize()
  }

  destroy() {
    cancelAnimationFrame(this.frameId)
    this.detachGestures()
  }

  resize() {
    const ratio = window.devicePixelRatio || 1
    this.canvas.width = Math.round(this.canvas.clientWidth * ratio)
    this.canvas.height = Math.round(this.canvas.clientHeight * ratio)
    this.camera.resize(this.canvas.clientWidth, this.canvas.clientHeight)
    this.requestFrame()
  }

  setVisible(visible) {
    this.visible = visible
    if (visible) this.requestFrame()
  }

  recolor() {
    this.colors = readThemeColors(this.canvas.parentElement)
    this.requestFrame()
  }

  requestFrame() {
    if (this.visible && this.camera.width) this.frameId ||= requestAnimationFrame(() => this.frame())
  }

  frame() {
    this.frameId = null
    if (this.running) this.running = this.layout.step() || this.dragging != null
    if (this.following) this.camera.fit(this.nodes)
    paintGraph(this.context, { nodes: this.nodes, links: this.links, colors: this.colors, camera: this.camera,
                               ratio: window.devicePixelRatio || 1, emphasis: this.emphasis })
    this.report()
    if (this.running) this.requestFrame()
  }

  report() {
    this.onState({ running: this.running, zoom: this.camera.k.toFixed(3), focus: this.walk[this.focusIndex] })
  }

  // O nó sob o ponto da tela, com uma folga de alguns px em volta do raio.
  nodeAt(px, py) {
    const [ wx, wy ] = this.camera.toWorld(px, py)
    const slop = HIT_SLOP_PX / this.camera.k
    const node = this.layout.nearest(wx, wy, (RADIUS_MIN + RADIUS_SPREAD) + slop)
    return node && Math.hypot(node.x - wx, node.y - wy) <= node.radius + slop ? node : null
  }

  hover(node) {
    if (node == this.hovered) return

    this.hovered = node
    this.canvas.toggleAttribute("data-hovering", node != null)
    this.emphasize()
  }

  grab(node) {
    this.dragging = node
    this.following = false
    this.pin(node, node.x, node.y)
    if (!this.reducedMotion) this.heat()
    this.requestFrame()
  }

  heat() {
    this.layout.heat()
    this.running = true
  }

  drag(node, px, py) {
    this.pin(node, ...this.camera.toWorld(px, py))
    this.requestFrame()
  }

  // O nó fica onde o ponteiro está; com a simulação parada (reduced-motion) é a posição que muda.
  pin(node, x, y) {
    Object.assign(node, { fx: x, fy: y, x, y })
  }

  release(node) {
    Object.assign(node, { fx: null, fy: null })
    this.dragging = null
    this.layout.cool()
    this.requestFrame()
  }

  open(node) {
    if (node.url) this.onOpen(node.url)
  }

  openFocused() {
    const node = this.walk[this.focusIndex]
    if (node) this.open(node)
  }

  zoomAt(px, py, factor) {
    this.following = false
    this.camera.zoomAt(px, py, factor)
    this.requestFrame()
  }

  zoom(direction) {
    this.zoomAt(this.camera.width / 2, this.camera.height / 2, ZOOM_STEP ** direction)
  }

  panBy(dx, dy) {
    this.following = false
    this.camera.panBy(dx, dy)
    this.requestFrame()
  }

  fit() {
    this.following = true
    this.requestFrame()
  }

  // j/k: a próxima nota (ou a anterior) na ordem do PageRank, presa nas pontas; fora da vista, a câmera vai até ela.
  step(direction) {
    const previous = this.walk[this.focusIndex]
    this.focusIndex = Math.min(Math.max(this.focusIndex + direction, 0), this.walk.length - 1)
    const node = this.walk[this.focusIndex]
    if (previous) previous.focused = false
    if (!node) return

    node.focused = true
    if (!this.camera.shows(node)) this.centerOn(node)
    this.emphasize()
  }

  centerOn(node) {
    this.following = false
    this.camera.centerOn(node.x, node.y)
  }

  emphasize() {
    this.emphasis.active = this.hovered ?? this.walk[this.focusIndex] ?? null
    this.requestFrame()
  }
}

function nodeOf({ id, title, url, hue, rank, unresolved, current }) {
  return { id, title, url, hue, rank, unresolved, current, neighbors: new Set(), radius: UNRESOLVED_RADIUS }
}

function connectNeighbors(links) {
  for (const { source, target } of links) {
    source.neighbors.add(target)
    target.neighbors.add(source)
  }
}

// Raio pela raiz do PageRank relativo ao maior, e o nome sempre visível nos mais centrais.
function markProminent(nodes) {
  const top = Math.max(...nodes.map((node) => node.rank ?? 0)) || 1
  nodes.filter((node) => !node.unresolved).forEach((node) => { node.radius = RADIUS_MIN + RADIUS_SPREAD * Math.sqrt((node.rank ?? 0) / top) })
  nodes.filter((node) => node.rank != null).sort(byRank).slice(0, PROMINENT_LABELS).forEach((node) => { node.prominent = true })
}

function byRank(a, b) {
  return (b.rank ?? -1) - (a.rank ?? -1)
}
