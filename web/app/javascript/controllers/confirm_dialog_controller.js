import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"

// Diálogo de confirmação do painel (layouts/_confirm_dialog) no lugar do confirm() do navegador: o Turbo pergunta por
// aqui a cada data-turbo-confirm e espera a resposta. O botão focado responde (o form method=dialog dá o returnValue);
// Esc ou clicar fora fecham sem ele, o que é não. Fechado, o foco volta a quem pediu. Quem pede pode trocar os dois
// rótulos (data-confirm-button e data-confirm-cancel): sair com mudança não salva pergunta com Leave e Stay. Com
// data-confirm-match, como o apagar do GitHub, o botão da ação só libera com o texto exato no campo.
export default class extends Controller {
  static targets = [ "message", "confirm", "cancel", "match", "matchLabel", "matchInput" ]
  static values = { defaultButton: String, defaultCancel: String }

  connect() {
    this.previousConfirm = Turbo.config.forms.confirm
    this.confirmHook = (message, _form, submitter) => this.ask(message, submitter)
    Turbo.config.forms.confirm = this.confirmHook
  }

  // Fora do layout do painel (telas públicas) o Turbo volta a perguntar como antes, sem um diálogo fora da página.
  disconnect() {
    if (Turbo.config.forms.confirm == this.confirmHook) Turbo.config.forms.confirm = this.previousConfirm
  }

  // Um item de menu que pede a confirmação fecha o menu, e o toggle que devolve o foco à linha vem numa tarefa depois:
  // esperar uma tarefa deixa o foco assentar em quem pediu antes de guardá-lo.
  async ask(message, submitter) {
    await new Promise((resolve) => setTimeout(resolve))
    this.opener = document.activeElement
    this.messageTarget.textContent = message
    const label = submitter?.dataset.confirmButton || this.defaultButtonValue
    this.confirmTarget.textContent = label
    this.cancelTarget.textContent = submitter?.dataset.confirmCancel || this.defaultCancelValue
    this.element.setAttribute("aria-label", label)
    this.element.returnValue = ""
    this.prepareMatch(submitter?.dataset)
    this.element.showModal()
    if (this.expected) this.matchInputTarget.focus()
    return new Promise((resolve) => (this.resolve = resolve))
  }

  answer() {
    this.resolve?.(this.element.returnValue == "confirm")
    this.resolve = null
    if (this.opener?.isConnected) this.opener.focus()
  }

  checkMatch() {
    this.confirmTarget.disabled = this.matchInputTarget.value !== this.expected
  }

  // Enter no campo não cai no envio implícito do form, que responderia com o Cancel, o primeiro botão.
  confirmMatch(event) {
    event.preventDefault()
    if (!this.confirmTarget.disabled) this.confirmTarget.click()
  }

  // Sem data-confirm-match o campo some e o botão fica livre, para a próxima pergunta não herdar a anterior.
  prepareMatch(dataset) {
    this.expected = dataset?.confirmMatch || null
    this.matchTarget.hidden = !this.expected
    this.matchLabelTarget.textContent = dataset?.confirmMatchLabel || ""
    this.matchInputTarget.value = ""
    this.confirmTarget.disabled = Boolean(this.expected)
  }
}
