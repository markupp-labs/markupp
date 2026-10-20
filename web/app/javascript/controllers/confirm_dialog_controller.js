import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"

// Diálogo de confirmação do painel (layouts/_confirm_dialog) no lugar do confirm() do navegador: o Turbo pergunta por
// aqui a cada data-turbo-confirm e espera a resposta. O botão focado responde (o form method=dialog dá o returnValue);
// Esc ou clicar fora fecham sem ele, o que é não. Fechado, o foco volta a quem pediu.
export default class extends Controller {
  static targets = [ "message", "confirm" ]
  static values = { defaultButton: String }

  connect() {
    Turbo.config.forms.confirm = (message, _form, submitter) => this.ask(message, submitter)
  }

  // Um item de menu que pede a confirmação fecha o menu, e o toggle que devolve o foco à linha vem numa tarefa depois:
  // esperar uma tarefa deixa o foco assentar em quem pediu antes de guardá-lo.
  async ask(message, submitter) {
    await new Promise((resolve) => setTimeout(resolve))
    this.opener = document.activeElement
    this.messageTarget.textContent = message
    this.confirmTarget.textContent = submitter?.dataset.confirmButton || this.defaultButtonValue
    this.element.returnValue = ""
    this.element.showModal()
    return new Promise((resolve) => (this.resolve = resolve))
  }

  answer() {
    this.resolve?.(this.element.returnValue == "confirm")
    this.resolve = null
    if (this.opener?.isConnected) this.opener.focus()
  }
}
