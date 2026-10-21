import { Controller } from "@hotwired/stimulus"
import { visitFrom } from "motion/busy_origin"
import { isTyping } from "controllers/typing"

// Paleta de comandos (layouts/_command_palette), como a do VS Code: Ctrl+Shift+P ou Cmd+Shift+P abre, F1 também (o
// Firefox reserva o Ctrl+Shift+P), e ":" fora de campo de texto, como o modo de comando do Vim. A busca fuzzy
// (as letras na ordem, com o que for entre elas) filtra, setas ou Ctrl+J e Ctrl+K (como no fzf; j e k sozinhos são
// letras da busca) andam, Enter ou clique roda, Esc fecha. Ação sem o que fazer na página (o modo da nota fora da nota)
// ou que o CSS esconde (o tema e o modo da nota já em uso, application.css) fica de fora. Cada item diz nos data-command-* como rodar (ShortcutsHelper).
const STEPS = { ArrowDown: 1, ArrowUp: -1, "ctrl+j": 1, "ctrl+k": -1 }

export default class extends Controller {
  static targets = [ "input", "option", "empty" ]

  openFromKey(event) {
    if (event.defaultPrevented || !opensPalette(event)) return

    event.preventDefault()
    this.open()
  }

  open() {
    this.optionTargets.forEach((option) => {
      option.hidden = false
      option.dataset.available = isAvailable(option)
    })
    this.inputTarget.value = ""
    this.filter()
    if (!this.element.open) this.element.showModal()
  }

  // Antes de o Turbo guardar a página que sai no cache: aberta, ela voltaria pelo histórico com a paleta à mostra.
  close() {
    this.element.close()
  }

  filter() {
    const query = this.inputTarget.value.toLowerCase()
    this.optionTargets.forEach((option) => (option.hidden = option.dataset.available != "true" || !fuzzyMatch(query, option.textContent)))
    this.emptyTarget.hidden = this.visibleOptions.length > 0
    this.select(this.visibleOptions[0])
  }

  // O Ctrl+K não sobe até a janela, onde abriria a busca de notas.
  navigate(event) {
    if (event.key == "Enter") return this.chooseSelected(event)
    const step = STEPS[event.ctrlKey ? `ctrl+${event.key}` : event.key]
    if (!step || event.altKey || event.metaKey) return

    event.preventDefault()
    event.stopPropagation()
    const options = this.visibleOptions
    this.select(options[(options.indexOf(this.selected) + step + options.length) % options.length])
  }

  chooseSelected(event) {
    event.preventDefault()
    if (this.selected) this.run(this.selected)
  }

  choose({ currentTarget }) {
    this.run(currentTarget)
  }

  select(option) {
    this.optionTargets.forEach((item) => item.setAttribute("aria-selected", String(item == option)))
    option ? this.inputTarget.setAttribute("aria-activedescendant", option.id) : this.inputTarget.removeAttribute("aria-activedescendant")
    option?.scrollIntoView({ block: "nearest" })
  }

  // A página escolhida carrega com a paleta aberta e a espera no item (motion/busy_origin.js); se a visita não começa
  // (a nota com mudança perguntou e o usuário ficou), fecha. O resto fecha antes de rodar: o foco volta para onde
  // estava, e é dali que a ação segue (o modo foca o editor).
  run(option) {
    const { commandClick, commandPress, commandHref, commandForm } = option.dataset
    if (commandHref) return visitFrom(option, commandHref) || this.element.close()

    this.element.close()
    if (commandClick) return document.querySelector(commandClick)?.click()
    if (commandPress) return document.body.dispatchEvent(new KeyboardEvent("keydown", { ...JSON.parse(commandPress), bubbles: true }))
    document.getElementById(commandForm)?.requestSubmit()
  }

  get selected() {
    return this.optionTargets.find((option) => option.getAttribute("aria-selected") == "true")
  }

  get visibleOptions() {
    return this.optionTargets.filter((option) => !option.hidden)
  }
}

// O Ctrl+Shift+P vem pelo code: com o Shift o key é "P", e em outro layout pode ser outra letra.
function opensPalette(event) {
  const ctrl = event.ctrlKey || event.metaKey
  if (event.altKey) return false
  if (event.key == "F1") return !ctrl && !event.shiftKey
  if (event.key == ":") return !ctrl && !isTyping(event.target)
  return event.code == "KeyP" && ctrl && event.shiftKey
}

// O display vale com a paleta fechada: o computado do item não depende do dialog em volta.
function isAvailable(option) {
  const selector = option.dataset.commandWhen
  return getComputedStyle(option).display != "none" && (!selector || document.querySelector(selector) != null)
}

// As letras da busca aparecem na ordem no nome (tgsd acha Toggle sidebar).
function fuzzyMatch(query, label) {
  let found = 0
  for (const char of label.toLowerCase()) if (char == query[found]) found++
  return found >= query.length
}
