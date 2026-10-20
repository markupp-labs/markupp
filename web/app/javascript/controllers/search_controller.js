import { Controller } from "@hotwired/stimulus"

const DEBOUNCE_MS = 200
// No campo, j e k são letras: andam as setas e, como na paleta (fzf), Ctrl+J e Ctrl+K. Na lista, setas e j e k.
const FIELD_STEPS = { ArrowDown: 1, ArrowUp: -1, "ctrl+KeyJ": 1, "ctrl+KeyK": -1 }
const LIST_STEPS = { ArrowDown: 1, ArrowUp: -1, KeyJ: 1, KeyK: -1 }

// Paleta de busca: o <dialog> abre com Ctrl+K, o form mira o turbo frame e a seleção anda pelo campo ou pela lista.
// Só o resultado selecionado entra no Tab, e o Enter nele segue o link.
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

  // O Ctrl+K no campo anda para cima e não chega ao da janela, que reabriria a busca.
  navigateField(event) {
    const step = FIELD_STEPS[event.ctrlKey ? `ctrl+${event.code}` : event.key]
    if (!step || event.altKey || event.metaKey || event.shiftKey) return

    event.preventDefault()
    event.stopPropagation()
    this.move(step)
  }

  navigateResults(event) {
    const step = LIST_STEPS[event.code]
    if (!step || event.ctrlKey || event.altKey || event.metaKey || event.shiftKey) return

    event.preventDefault()
    this.move(step)?.focus()
  }

  choose(event) {
    const selected = this.resultTargets.find((result) => result.ariaSelected === "true")
    if (!selected) return

    event.preventDefault()
    selected.click()
  }

  move(step) {
    const results = this.resultTargets
    if (results.length === 0) return

    const current = results.findIndex((result) => result.ariaSelected === "true")
    const target = (current + step + results.length) % results.length
    results.forEach((result, index) => {
      result.ariaSelected = String(index === target)
      result.tabIndex = index === target ? 0 : -1
    })
    results[target].scrollIntoView({ block: "nearest" })
    return results[target]
  }
}
