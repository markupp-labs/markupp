import { Controller } from "@hotwired/stimulus"

// Trecho da página Agents (settings/agents): o botão copia o texto do pre e o status diz o resultado por 2 segundos.
const STATUS_TIMEOUT = 2000

export default class extends Controller {
  static targets = [ "source", "status" ]
  static values = { copied: String, copyFailed: String }

  async copy() {
    try {
      await navigator.clipboard.writeText(this.sourceTarget.textContent)
      this.statusTarget.textContent = this.copiedValue
    } catch {
      this.statusTarget.textContent = this.copyFailedValue
    }
    clearTimeout(this.statusTimer)
    this.statusTimer = setTimeout(() => (this.statusTarget.textContent = ""), STATUS_TIMEOUT)
  }
}
