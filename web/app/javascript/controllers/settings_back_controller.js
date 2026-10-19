import { Controller } from "@hotwired/stimulus"

// Esc nas configurações volta para a página de origem, como sair de um painel no VS Code. Esc dentro de campo,
// diálogo ou popover aberto é deles (fechar, desfazer), então fica de fora.
export default class extends Controller {
  go(event) {
    if (event.defaultPrevented || this.claimedByOther(event.target)) return

    this.element.click()
  }

  claimedByOther(target) {
    return target.closest("input, textarea, select, [contenteditable]") != null ||
      document.querySelector("dialog[open], :popover-open") != null
  }
}
