import { Controller } from "@hotwired/stimulus"
import { playOnce } from "motion/play_once"
import { inVimEditor } from "controllers/typing"

// Barra lateral estilo VS Code. No desktop recolhe/expande (Ctrl+B ou Cmd+B) e redimensiona pela alça na borda;
// a barra de atividades troca a visão dela (Explorer, Links, Graph). Estado, largura e visão vão para cookies
// que o servidor lê no próximo render (SidebarHelper). No celular é a gaveta popover, com a barra de atividades no topo.
const COOKIE_MAX_AGE = 60 * 60 * 24 * 365
const KEY_STEP = 16
// Atalho de cada visão, como no VS Code (Ctrl+Shift+E e F); Links e Graph pegam L e G, livres no navegador e no editor.
const VIEW_KEYS = { KeyE: "explorer", KeyL: "links", KeyG: "graph" }
// Setas e j/k (h/l na linha da gaveta) andam entre os ícones da barra de atividades, dando a volta nas pontas; teclas pelo code.
const TAB_STEPS = { ArrowDown: 1, ArrowRight: 1, KeyJ: 1, KeyL: 1, ArrowUp: -1, ArrowLeft: -1, KeyK: -1, KeyH: -1 }
const desktop = window.matchMedia("(width >= 48rem)")

export default class extends Controller {
  static targets = [ "panel", "handle", "toggle", "tab", "view" ]
  static values = { hideLabel: String, showLabel: String }

  // Atalho vale também com o foco no CodeMirror. Se o editor já tratou a tecla (no Mac, Ctrl+B é o
  // cursor para a esquerda, à moda do Emacs), ele vence e a barra fica como está; com o modo Vim, o Ctrl+B é dele.
  toggleFromShortcut(event) {
    if (event.defaultPrevented || inVimEditor(event.target)) return

    event.preventDefault()
    this.toggle()
  }

  // Ctrl+Shift+E, L, G e F (Cmd no Mac) mostram a visão, abrindo a barra fechada, e levam o foco a ela: no Explorer à
  // linha da árvore, como no VS Code. A tecla vem pelo code: com o Shift o key é maiúsculo, e em outro layout pode ser outra letra.
  focusView(event) {
    const view = viewOfShortcut(event)
    const panel = this.viewTargets.find((candidate) => candidate.dataset.sidebarView == view)
    const target = panel && focusTargetOf(panel)
    if (!target) return

    event.preventDefault()
    this.openView(view)
    target.focus()
  }

  // Mostra a visão, abrindo a barra (ou a gaveta) fechada.
  openView(view) {
    this.showView(view)
    if (this.isHidden) this.toggle()
  }

  // Clique num ícone da barra de atividades, como no VS Code: o da visão à vista recolhe a barra (no celular, fecha a
  // gaveta); outro troca a visão e, com a barra recolhida, a abre nela.
  selectView({ params: { view } }) {
    if (!this.isHidden && view == this.currentView) return this.toggle()

    this.showView(view)
    if (desktop.matches && this.isHidden) this.toggle()
  }

  showView(view) {
    this.viewTargets.forEach((panel) => { panel.hidden = panel.dataset.sidebarView != view })
    this.tabTargets.forEach((tab) => {
      const selected = tab.dataset.sidebarViewParam == view
      tab.setAttribute("aria-selected", String(selected))
      tab.tabIndex = selected ? 0 : -1
    })
    writeCookie("sidebar_view", view)
  }

  // Só o ícone com o foco fica no Tab (roving tabindex); Enter ou espaço mostram a visão dele pelo clique do botão.
  moveTab(event) {
    const step = TAB_STEPS[event.code]
    if (!step || event.altKey || event.ctrlKey || event.metaKey) return

    event.preventDefault()
    const tabs = [ ...event.currentTarget.parentElement.children ]
    const next = tabs[(tabs.indexOf(event.currentTarget) + step + tabs.length) % tabs.length]
    tabs.forEach((tab) => { tab.tabIndex = tab == next ? 0 : -1 })
    next.focus()
  }

  get currentView() {
    return this.viewTargets.find((panel) => !panel.hidden)?.dataset.sidebarView
  }

  get isHidden() {
    return desktop.matches ? document.documentElement.dataset.sidebar == "closed" : !this.panelTarget.matches(":popover-open")
  }

  toggle() {
    if (!desktop.matches) return this.panelTarget.togglePopover()

    const closing = document.documentElement.dataset.sidebar != "closed"
    // Só o clique desliza a barra (motion.css), uma vez; carregar, navegar ou arrastar a alça não anima.
    playOnce(this.element, "data-sidebar-moved", () => {
      if (closing) document.documentElement.dataset.sidebar = "closed"
      else delete document.documentElement.dataset.sidebar
    })
    writeCookie("sidebar", closing ? "closed" : "open")
    this.labelToggle(closing)
    // Quem tinha o foco some junto com a barra: o foco passa ao botão, que fica.
    if (closing && this.panelTarget.contains(document.activeElement)) this.toggleTarget.focus()
  }

  // O botão diz o que vai fazer: esconder a barra aberta ou mostrar a fechada.
  labelToggle(closed) {
    const label = closed ? this.showLabelValue : this.hideLabelValue
    this.toggleTarget.setAttribute("aria-expanded", String(!closed))
    this.toggleTarget.setAttribute("aria-label", label)
    this.toggleTarget.title = `${label} (Ctrl+B)`
  }

  startResize(event) {
    event.preventDefault()
    this.handleTarget.setPointerCapture(event.pointerId)
    this.handleTarget.dataset.dragging = ""
    this.dragStart = { x: event.clientX, width: this.width }
  }

  resize(event) {
    if (!this.dragStart) return

    this.applyWidth(this.dragStart.width + event.clientX - this.dragStart.x)
  }

  endResize() {
    if (!this.dragStart) return

    this.dragStart = null
    delete this.handleTarget.dataset.dragging
    writeCookie("sidebar_width", this.width)
  }

  resizeByKey(event) {
    const step = { ArrowLeft: -KEY_STEP, ArrowRight: KEY_STEP }[event.key]
    if (!step) return

    event.preventDefault()
    this.applyWidth(this.width + step)
    writeCookie("sidebar_width", this.width)
  }

  applyWidth(width) {
    const { valueMin, valueMax } = this.limits
    const clamped = Math.round(Math.min(Math.max(width, valueMin), valueMax))
    document.documentElement.style.setProperty("--sidebar-width", `${clamped}px`)
    this.handleTarget.setAttribute("aria-valuenow", clamped)
  }

  get width() {
    return Number(this.handleTarget.getAttribute("aria-valuenow"))
  }

  // Os limites vêm do servidor nos aria-valuemin/max da alça, uma fonte só.
  get limits() {
    return { valueMin: Number(this.handleTarget.getAttribute("aria-valuemin")), valueMax: Number(this.handleTarget.getAttribute("aria-valuemax")) }
  }
}

function viewOfShortcut(event) {
  if (!(event.ctrlKey || event.metaKey) || !event.shiftKey || event.altKey || event.defaultPrevented) return null
  return VIEW_KEYS[event.code]
}

// Primeiro o lugar que a visão pede (data-sidebar-view-focus); sem ele, a visão que aceita o foco (tabindex -1).
function focusTargetOf(panel) {
  return panel.querySelector(panel.dataset.sidebarViewFocus) ?? (panel.hasAttribute("tabindex") ? panel : null)
}

function writeCookie(name, value) {
  document.cookie = `${name}=${value}; path=/; max-age=${COOKIE_MAX_AGE}; samesite=lax`
}
