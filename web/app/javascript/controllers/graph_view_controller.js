import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"
import { replaceQuery } from "controllers/url_query"

// Grafo em canvas (graphs/_canvas), na visão Graph da barra lateral e na tela cheia do cofre. Só quando o canvas fica à
// vista o controller baixa a cena (graph/scene.js, que traz o d3-force) e pede o json; antes disso nada baixa. Fora da
// vista a cena pausa. O canvas tem o foco: j/k e as setas andam pelas notas do mais central ao menos, Enter abre, + e -
// dão zoom e 0 enquadra, teclas pelo code. O elemento diz em data-graph-nodes, -links, -running, -zoom e -focus o que
// está desenhado (os testes leem dali); o status diz o mesmo em texto, e o aviso falado diz a nota com o foco. Uma lista
// só para o leitor de tela (sr-only) traz um link por nota desenhada, refeita a cada desenho, com os filtros.
const ACTIONS = {
  KeyJ: [ "step", 1 ], ArrowDown: [ "step", 1 ], KeyK: [ "step", -1 ], ArrowUp: [ "step", -1 ], Enter: [ "openFocused" ],
  Equal: [ "zoom", 1 ], NumpadAdd: [ "zoom", 1 ], Minus: [ "zoom", -1 ], NumpadSubtract: [ "zoom", -1 ], Digit0: [ "fit" ], Numpad0: [ "fit" ]
}
const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)")
const darkScheme = window.matchMedia("(prefers-color-scheme: dark)")

export default class extends Controller {
  static targets = [ "canvas", "empty", "filtered", "refused", "failure", "status", "announce", "list" ]
  static values = { url: String, autofocus: Boolean, filtered: Boolean, labels: Object }

  connect() {
    this.visibility = new IntersectionObserver(([ entry ]) => this.visibilityChanged(entry.isIntersecting))
    this.visibility.observe(this.canvasTarget)
    this.sizing = new ResizeObserver(() => this.scene?.resize())
    this.sizing.observe(this.canvasTarget)
    this.recolor = () => this.scene?.recolor()
    this.themeObserver = new MutationObserver(this.recolor)
    this.themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: [ "data-theme" ] })
    darkScheme.addEventListener("change", this.recolor)
    if (this.autofocusValue) this.canvasTarget.focus()
  }

  // O pedido em andamento fica sem dono (draw descarta) e uma reconexão volta a desenhar.
  disconnect() {
    this.request = null
    this.drawn = null
    for (const observer of [ this.visibility, this.sizing, this.themeObserver ]) observer.disconnect()
    darkScheme.removeEventListener("change", this.recolor)
    this.scene?.destroy()
    this.scene = null
  }

  visibilityChanged(visible) {
    if (visible) this.drawn ??= this.draw(this.urlValue)
    this.scene?.setVisible(visible)
  }

  changeDepth({ currentTarget, params: { depth } }) {
    this.element.querySelectorAll("[data-graph-view-depth-param]").forEach((button) => button.setAttribute("aria-pressed", String(button == currentTarget)))
    this.element.dataset.graphDepth = depth
    this.redraw({ depth })
  }

  // Pede de novo o grafo com a query trocada; valor null tira o parâmetro.
  redraw(query) {
    this.drawn = this.draw(replaceQuery(new URL(this.urlValue, window.location.href), query))
  }

  press(event) {
    const [ action, argument ] = ACTIONS[event.code] ?? []
    if (!action || event.target != this.canvasTarget || event.ctrlKey || event.metaKey || event.altKey || !this.scene) return

    event.preventDefault()
    this.scene[action](argument)
  }

  // Pedidos que se cruzam (trocar a profundidade antes de o grafo chegar): só o último desenha.
  async draw(url) {
    const request = this.request = Symbol("graph request")
    this.element.setAttribute("aria-busy", "true")
    try {
      const [ graph, { createGraphScene } ] = await Promise.all([ fetchGraph(url), import("graph/scene") ])
      if (request == this.request) this.show(graph, createGraphScene)
    } catch (error) {
      if (request == this.request) this.showFailure(error, url)
    } finally {
      if (request == this.request) this.element.removeAttribute("aria-busy")
    }
  }

  show(graph, createGraphScene) {
    this.scene?.destroy()
    this.scene = null
    this.reveal(graph.nodes.length ? null : this.emptyOverlay())
    Object.assign(this.element.dataset, { graphNodes: graph.nodes.length, graphLinks: graph.links.length })
    this.statusTarget.textContent = this.statusOf(graph)
    this.listNotes([])
    if (!graph.nodes.length) return

    this.scene = createGraphScene(this.canvasTarget, graph, {
      reducedMotion: reducedMotion.matches, onState: (state) => this.mirror(state), onOpen: (url) => Turbo.visit(url)
    })
    this.listNotes(this.scene.walk)
  }

  // A lista do leitor de tela: um link por nota desenhada, na ordem do j, fora do Tab (o canvas anda pelas notas).
  listNotes(notes) {
    this.listTarget.replaceChildren(...notes.map(({ title, url }) => {
      const item = document.createElement("li")
      item.append(Object.assign(document.createElement("a"), { href: url, textContent: title, tabIndex: -1 }))
      return item
    }))
  }

  showFailure(error, url) {
    console.error(JSON.stringify({ event: "markupp_graph_load_failed", url: String(url), status: error.status ?? null, message: error.message }))
    this.reveal(error.status == 403 ? this.refusedTarget : this.failureTarget)
    this.listNotes([])
    this.statusTarget.textContent = ""
  }

  // Botão do vazio com filtro: o foco volta ao canvas, que o botão some, e os filtros (graph-filters) se limpam.
  clearFilters() {
    this.canvasTarget.focus()
    this.dispatch("clear-filters")
  }

  // Sem notas: com filtro ligado, o vazio que diz isso; senão o do cofre.
  emptyOverlay() {
    return this.filteredValue && this.hasFilteredTarget ? this.filteredTarget : this.emptyTarget
  }

  // Um aviso no lugar do canvas (vazio, vazio pelos filtros, recusado, sem servidor), ou nenhum.
  reveal(overlay) {
    [ this.emptyTarget, ...this.filteredTargets, this.refusedTarget, this.failureTarget ].forEach((target) => { target.hidden = target != overlay })
  }

  mirror({ running, zoom, focus }) {
    Object.assign(this.element.dataset, { graphRunning: running, graphZoom: zoom })
    if (focus?.id == this.element.dataset.graphFocus) return

    this.element.dataset.graphFocus = focus?.id ?? ""
    this.announceTarget.textContent = focus?.title ?? ""
  }

  statusOf({ nodes, links, total }) {
    const label = total > nodes.length ? this.labelsValue.capped : this.labelsValue.status
    return label.replace("%{nodes}", nodes.length).replace("%{shown}", nodes.length).replace("%{total}", total).replace("%{links}", links.length)
  }
}

async function fetchGraph(url) {
  const response = await fetch(url, { headers: { Accept: "application/json" } })
  if (!response.ok) throw Object.assign(new Error(`graph answered ${response.status}`), { status: response.status })

  return response.json()
}
