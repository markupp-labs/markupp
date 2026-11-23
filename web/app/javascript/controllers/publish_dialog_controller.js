import { Controller } from "@hotwired/stimulus"

// Diálogo de publicar (layouts/_publish_dialog): o link que mira o frame dele abre o diálogo antes da prévia chegar, e o
// frame mostra a espera. Chegou a prévia, o foco vai ao campo do nome; o Publish só libera com o nome exato, como o
// apagar do GitHub (o servidor confere de novo). Fechado, o frame volta à espera e o foco volta a quem abriu.
export default class extends Controller {
  static targets = [ "frame", "waiting", "name", "submit" ]

  show() {
    if (this.element.open) return

    this.opener = document.activeElement
    this.element.showModal()
  }

  nameTargetConnected(field) {
    field.focus()
    this.check()
  }

  check() {
    this.submitTarget.disabled = this.nameTarget.value !== this.nameTarget.dataset.expected
  }

  restoreFocus() {
    this.frameTarget.replaceChildren(this.waitingTarget.content.cloneNode(true))
    if (this.opener?.isConnected) this.opener.focus()
  }
}
