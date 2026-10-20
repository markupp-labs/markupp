import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"

// Alt+G (Option+G no Mac, pela tecla e não pelo caractere) abre o grafo do cofre em tela cheia, de qualquer lugar,
// como o Alt+N da nota nova. Com uma nota aberta, a tela parte dela (o link da visão Graph, data-full-graph-link): a nota
// fica em destaque e o Esc volta a ela. Chrome, Firefox e o CodeMirror não ligam Alt+G.
export default class extends Controller {
  static values = { url: String }

  open(event) {
    if (event.code != "KeyG" || !event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || event.defaultPrevented) return

    event.preventDefault()
    Turbo.visit(document.querySelector("[data-full-graph-link]")?.href ?? this.urlValue)
  }
}
