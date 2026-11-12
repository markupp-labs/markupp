import { Controller } from "@hotwired/stimulus"

// Olho ao lado do campo de senha: mostra e esconde o texto, e o aria-pressed diz o estado. No envio a senha volta a ficar
// escondida, para o navegador tratar o campo como senha ao oferecer guardá-la.
export default class extends Controller {
  static targets = [ "field" ]

  connect() {
    this.hide = this.hide.bind(this)
    this.element.closest("form")?.addEventListener("submit", this.hide)
  }

  disconnect() {
    this.element.closest("form")?.removeEventListener("submit", this.hide)
  }

  toggle() {
    this.show(this.fieldTarget.type === "password")
  }

  hide() {
    this.show(false)
  }

  show(visible) {
    this.fieldTarget.type = visible ? "text" : "password"
    this.element.querySelector("[aria-pressed]").setAttribute("aria-pressed", String(visible))
  }
}
