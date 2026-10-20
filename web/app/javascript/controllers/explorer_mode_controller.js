import { Controller } from "@hotwired/stimulus"

// Seletor da visão do Explorer (notes/_explorer_mode_picker): trocar envia o form ao frame explorer_list, que volta com
// a lista e grava a escolha no cookie. Alt+O (pelo code: no Mac o Option muda a letra) de qualquer lugar foca o seletor
// e abre a lista de visões; com o Explorer escondido não faz nada.
export default class extends Controller {
  static targets = [ "select" ]

  submit() {
    this.element.requestSubmit()
  }

  open(event) {
    if (event.code != "KeyO" || !event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || event.defaultPrevented) return
    if (!this.selectTarget.checkVisibility()) return

    event.preventDefault()
    this.selectTarget.focus()
    try {
      this.selectTarget.showPicker()
    } catch {
      // Sem showPicker no select (navegador antigo), fica o foco: as setas trocam a visão.
    }
  }
}
