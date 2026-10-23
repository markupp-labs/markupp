import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"
import { isTyping } from "controllers/typing"

// g e depois t, fora de campo de texto, abre a lista de tenants; g e depois v, o seletor de cofres da trilha, com o foco
// no primeiro cofre (context_menu_controller.js). Atalho em sequência como o gt do Vim e o g n do GitHub, sem disputar
// Ctrl ou Alt com o navegador. Qualquer outra tecla no meio desfaz o g.
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
    if (afterG && event.key == "v") return this.openVaults(event)
    this.afterG = event.key == "g"
  }

  // O click de teclado (detail 0) leva o foco ao primeiro item, como o Enter no nome do cofre.
  openVaults(event) {
    const opener = document.querySelector("button[popovertarget=vault_switcher]")
    if (!opener) return
    event.preventDefault()
    opener.click()
  }
}
