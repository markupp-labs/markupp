import { Controller } from "@hotwired/stimulus"
import { isTyping } from "controllers/typing"

// Esc numa área ao lado das notas (AsideArea) volta para a página de origem, como sair de um painel no VS Code. Esc
// dentro de campo, diálogo, popover ou camada aberta (data-esc-layer, como os filtros do grafo) é deles (fechar,
// desfazer), então fica de fora.
export default class extends Controller {
  go(event) {
    if (event.defaultPrevented || this.claimedByOther(event.target)) return

    this.element.click()
  }

  claimedByOther(target) {
    return isTyping(target) || document.querySelector("dialog[open], :popover-open, [data-esc-layer][open]") != null
  }
}
