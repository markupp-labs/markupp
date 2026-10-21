import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"

// Ctrl+, ou Cmd+, abre as configurações de qualquer tela logada, como no VS Code e no Obsidian.
// Nota com alteração não salva pergunta antes de sair (turbo:before-visit do note_form_controller).
export default class extends Controller {
  static values = { url: String }

  open(event) {
    if (event.key != "," || !(event.ctrlKey || event.metaKey) || event.defaultPrevented) return

    event.preventDefault()
    Turbo.visit(this.urlValue)
  }
}
