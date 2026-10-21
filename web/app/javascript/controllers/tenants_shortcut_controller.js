import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"
import { isTyping } from "controllers/typing"

// g e depois t, fora de campo de texto, abre a lista de tenants: atalho em sequência como o gt do Vim e o g n do
// GitHub, sem disputar Ctrl ou Alt com o navegador. Qualquer outra tecla no meio desfaz o g.
export default class extends Controller {
  static values = { url: String }

  open(event) {
    const afterG = this.afterG
    this.afterG = false
    if (event.ctrlKey || event.metaKey || event.altKey || event.defaultPrevented || isTyping(event.target)) return
    if (afterG && event.key == "t") {
      event.preventDefault()
      return Turbo.visit(this.urlValue)
    }
    this.afterG = event.key == "g"
  }
}
