import { Controller } from "@hotwired/stimulus"

// Barra lateral estilo VS Code. No desktop recolhe/expande (Ctrl+B ou Cmd+B) e redimensiona pela alça na borda;
// estado e largura vão para cookies que o servidor lê no próximo render (SidebarHelper). No celular é a gaveta popover.
const COOKIE_MAX_AGE = 60 * 60 * 24 * 365
const KEY_STEP = 16
const desktop = window.matchMedia("(width >= 48rem)")

export default class extends Controller {
  static targets = [ "panel", "handle", "toggle" ]
  static values = { hideLabel: String, showLabel: String }

  // Atalho vale também com o foco no CodeMirror. Se o editor já tratou a tecla (no Mac, Ctrl+B é o
  // cursor para a esquerda, à moda do Emacs), ele vence e a barra fica como está.
  toggleFromShortcut(event) {
    if (event.defaultPrevented) return

    event.preventDefault()
    this.toggle()
  }

  toggle() {
    if (!desktop.matches) return this.panelTarget.togglePopover()

    const closing = document.documentElement.dataset.sidebar != "closed"
    if (closing) document.documentElement.dataset.sidebar = "closed"
    else delete document.documentElement.dataset.sidebar
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

function writeCookie(name, value) {
  document.cookie = `${name}=${value}; path=/; max-age=${COOKIE_MAX_AGE}; samesite=lax`
}
