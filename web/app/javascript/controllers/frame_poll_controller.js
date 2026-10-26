import { Controller } from "@hotwired/stimulus"

// Recarrega um turbo-frame já carregado (o próprio elemento ou o target frame): a cada interval ms (0 desliga) e quando
// a aba volta a ficar visível. Com a aba oculta, ou com o frame fora da tela (a barra que o layout esconde, o popover
// fechado), não pede nada. O reload() também serve de action, como no toggle do popover, que só recarrega ao abrir.
// O frame ainda não carregado fica com o loading lazy dele. ADR-0049: o sino recarrega o contador a cada 60s.
export default class extends Controller {
  static targets = [ "frame" ]
  static values = { interval: { type: Number, default: 60000 } }

  connect() {
    this.reloadWhenShown = () => { if (!document.hidden) this.reload() }
    document.addEventListener("visibilitychange", this.reloadWhenShown)
    if (this.intervalValue > 0) this.timer = setInterval(() => this.reload(), this.intervalValue)
  }

  disconnect() {
    document.removeEventListener("visibilitychange", this.reloadWhenShown)
    clearInterval(this.timer)
  }

  reload(event) {
    if (event?.newState == "closed" || !this.reloadable) return

    this.frame.reload()
  }

  get reloadable() {
    return !document.hidden && this.frame.hasAttribute("complete") && this.frame.checkVisibility()
  }

  get frame() {
    return this.hasFrameTarget ? this.frameTarget : this.element
  }
}
