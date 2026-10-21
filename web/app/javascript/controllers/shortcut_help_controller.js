import { Controller } from "@hotwired/stimulus"
import { isTyping } from "controllers/typing"

// "?" fora de campo de texto abre a ajuda de atalhos (layouts/_shortcut_help), como no GitHub e no Gmail. No editor, no
// caminho e nas buscas o ? é texto. A paleta abre a ajuda repetindo a tecla no body.
export default class extends Controller {
  openFromKey(event) {
    if (event.key != "?" || event.ctrlKey || event.metaKey || event.altKey || event.defaultPrevented || isTyping(event.target)) return

    event.preventDefault()
    if (!this.element.open) this.element.showModal()
  }
}
