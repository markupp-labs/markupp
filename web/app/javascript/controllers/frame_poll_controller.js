import { Controller } from "@hotwired/stimulus"

// Recarrega um turbo-frame já carregado (o próprio elemento ou o target frame): a cada interval ms (0 desliga) e quando
// a aba volta a ficar visível. Com a aba oculta, com o frame fora da tela (a barra que o layout esconde, o popover
// fechado) ou com um pedido ainda em curso, não pede nada. O reload() também serve de action, como no toggle do
// popover, que só recarrega ao abrir. O frame ainda não carregado fica com o loading lazy dele. ADR-0049: o sino
// recarrega o contador a cada 60s.
//
// "Já carregado" é um turbo:frame-load visto, não o atributo complete: o Turbo tira o complete no início de cada
// reload e não o devolve quando o fetch falha (rede caída), o que pararia o polling de vez. Resposta sem o frame (a
// página de erro do proxy num deploy) não troca o conteúdo: o frame fica com o que mostrava, sem o "Content missing"
// do Turbo, e o próximo ciclo tenta de novo.
export default class extends Controller {
  static targets = [ "frame" ]
  static values = { interval: { type: Number, default: 60000 } }

  connect() {
    this.loaded = this.frame.hasAttribute("complete")
    this.markLoaded = (event) => { if (event.target == this.frame) this.loaded = true }
    this.keepContent = (event) => { if (event.target == this.frame) event.preventDefault() }
    this.reloadWhenShown = () => { if (!document.hidden) this.reload() }
    this.listen("addEventListener")
    if (this.intervalValue > 0) this.timer = setInterval(() => this.reload(), this.intervalValue)
  }

  disconnect() {
    this.listen("removeEventListener")
    clearInterval(this.timer)
  }

  listen(method) {
    this.element[method]("turbo:frame-load", this.markLoaded)
    this.element[method]("turbo:frame-missing", this.keepContent)
    document[method]("visibilitychange", this.reloadWhenShown)
  }

  reload(event) {
    if (event?.newState == "closed" || !this.reloadable) return

    this.frame.reload()
  }

  get reloadable() {
    return !document.hidden && this.loaded && !this.frame.hasAttribute("busy") && this.frame.checkVisibility()
  }

  get frame() {
    return this.hasFrameTarget ? this.frameTarget : this.element
  }
}
