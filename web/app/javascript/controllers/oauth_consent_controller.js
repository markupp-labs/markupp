import { Controller } from "@hotwired/stimulus"

// Teclado do consentimento OAuth (oauth_authorizations/show): Esc recusa e Enter aprova de qualquer ponto da tela. Enter
// num botão ou link segue o próprio elemento, como o Tab até o Deny espera.
export default class extends Controller {
  static targets = [ "approve", "deny" ]

  answer(event) {
    if (event.defaultPrevented || event.repeat || event.isComposing) return
    if (event.key === "Escape") return this.press(event, this.denyTarget)
    if (event.key === "Enter" && !event.target.closest("button, a, input, select, textarea")) this.press(event, this.approveTarget)
  }

  press(event, button) {
    event.preventDefault()
    button.click()
  }
}
