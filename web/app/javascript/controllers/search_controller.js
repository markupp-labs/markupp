import { Controller } from "@hotwired/stimulus"

const DEBOUNCE_MS = 200

// Paleta de busca: o <dialog> abre com Ctrl+K, o form mira o turbo frame e as setas movem a seleção.
export default class extends Controller {
  static targets = [ "dialog", "form", "input", "result" ]

  open() {
    if (!this.dialogTarget.open) this.dialogTarget.showModal()
    this.inputTarget.select()
  }

  submit() {
    clearTimeout(this.timer)
    this.timer = setTimeout(() => this.formTarget.requestSubmit(), DEBOUNCE_MS)
  }

  next() { this.move(1) }

  previous() { this.move(-1) }

  choose(event) {
    const selected = this.resultTargets.find((result) => result.ariaSelected === "true")
    if (!selected) return

    event.preventDefault()
    selected.click()
  }

  move(step) {
    const results = this.resultTargets
    const current = results.findIndex((result) => result.ariaSelected === "true")
    const target = (current + step + results.length) % results.length
    results.forEach((result, index) => result.ariaSelected = String(index === target))
    results[target]?.scrollIntoView({ block: "nearest" })
  }
}
