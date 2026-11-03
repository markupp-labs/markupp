import { Controller } from "@hotwired/stimulus"

// Botão que copia um link (o público do cofre, na seção Publication) e diz o resultado no status por 2 segundos, que o
// leitor de tela lê pelo aria-live.
const STATUS_TIMEOUT = 2000

export default class extends Controller {
  static targets = [ "status" ]
  static values = { text: String, copied: String, failed: String }

  disconnect() {
    clearTimeout(this.statusTimer)
  }

  async copy() {
    try {
      await navigator.clipboard.writeText(this.textValue)
      this.say(this.copiedValue)
    } catch {
      this.say(this.failedValue)
    }
  }

  say(message) {
    this.statusTarget.textContent = message
    clearTimeout(this.statusTimer)
    this.statusTimer = setTimeout(() => (this.statusTarget.textContent = ""), STATUS_TIMEOUT)
  }
}
