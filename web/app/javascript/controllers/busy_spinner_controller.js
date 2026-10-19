import { Controller } from "@hotwired/stimulus"

const SHOW_AFTER_MS = 300

// Indicador de carregamento (helpers/busy_spinner_helper.rb): espera rápida não pisca nada; passando de 300ms entra o
// glifo braille, que gira pelo CSS (motion.css), e o rótulo, que o role=status anuncia. busyWhen é um seletor: há espera
// enquanto ele achar algo (o Turbo põe e tira o aria-busy). Sem busyWhen, o indicador existir já é a espera.
export default class extends Controller {
  static values = { busyWhen: String, label: String }

  connect() {
    this.observer = new MutationObserver(() => this.sync())
    this.observer.observe(document.documentElement, { attributes: true, attributeFilter: [ "aria-busy" ], subtree: true })
    this.sync()
  }

  disconnect() {
    this.observer.disconnect()
    this.hide()
  }

  sync() {
    if (this.busy) this.showTimer ??= setTimeout(() => this.show(), SHOW_AFTER_MS)
    else this.hide()
  }

  get busy() {
    return !this.hasBusyWhenValue || document.querySelector(this.busyWhenValue) != null
  }

  show() {
    const glyph = Object.assign(document.createElement("span"), { className: "busy-spinner-glyph" })
    glyph.setAttribute("aria-hidden", "true")
    const label = Object.assign(document.createElement("span"), { className: "busy-spinner-label", textContent: this.labelValue })
    this.element.replaceChildren(glyph, label)
  }

  hide() {
    clearTimeout(this.showTimer)
    this.showTimer = null
    this.element.replaceChildren()
  }
}
