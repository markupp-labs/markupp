import { Controller } from "@hotwired/stimulus"
import { isTyping } from "controllers/typing"

// Atalho de uma tecla que age no próprio elemento: campo ganha o foco (o / da busca pública, como no GitHub); botão ou
// link é clicado (o Alt+P de publicar a nota). Sem Alt a tecla vale fora de campo de texto e casa pelo caractere (key);
// com Alt vale em qualquer lugar e casa pela tecla (code), porque no Mac o Option muda o caractere.
export default class extends Controller {
  static values = { key: String, code: String, alt: Boolean }

  press(event) {
    if (!this.matches(event) || event.defaultPrevented) return

    event.preventDefault()
    // O foco vai antes ao botão ou link, para quem ele abrir (o diálogo de publicar) devolver o foco a ele.
    if (!this.element.matches("input, textarea")) return this.element.focus() || this.element.click()
    this.element.focus()
    this.element.select()
  }

  matches(event) {
    if (event.ctrlKey || event.metaKey || event.altKey != this.altValue) return false

    return this.altValue ? event.code == this.codeValue && !event.shiftKey : event.key == this.keyValue && !isTyping(event.target)
  }
}
