import { Controller } from "@hotwired/stimulus"
import { renderMarkdown } from "editor/render_markdown"
import { resolveWikilink } from "editor/wikilinks"

// Modos da nota: edição (CodeMirror), leitura (markdown renderizado) e lado a lado (preview a cada tecla).
// O modo vai no data-mode do form (o CSS mostra e esconde os painéis) e no cookie note_mode (NoteModeHelper).
// Atalhos: Ctrl+E alterna edição e leitura, como no Obsidian; Ctrl+\ alterna o lado a lado, como o split do VS Code.
const COOKIE_MAX_AGE = 60 * 60 * 24 * 365
const SHORTCUTS = { e: "read", "\\": "split" }

export default class extends Controller {
  static targets = [ "source", "preview", "button" ]
  static values = { wikilinks: Object, folder: String, missingNote: String }

  connect() {
    this.render()
  }

  choose({ params }) {
    this.apply(params.mode)
  }

  // Respeita quem já tratou a tecla (no Mac o CodeMirror liga Ctrl+E ao fim da linha; ali vale Cmd+E).
  shortcut(event) {
    const target = SHORTCUTS[event.key]
    if (!target || !(event.ctrlKey || event.metaKey) || event.altKey || event.defaultPrevented) return

    event.preventDefault()
    this.apply(this.mode == target ? "edit" : target)
  }

  apply(mode) {
    this.element.dataset.mode = mode
    this.buttonTargets.forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.noteModeModeParam == mode)))
    document.cookie = `note_mode=${mode}; path=/; max-age=${COOKIE_MAX_AGE}; samesite=lax`
    this.render()
    if (mode != "read") this.element.querySelector(".cm-content")?.focus()
  }

  // Em edição o preview fica escondido e não precisa ser desenhado a cada tecla.
  render() {
    if (this.mode == "edit") return

    this.previewTarget.innerHTML = renderMarkdown(this.sourceTarget.value, { wikilinks: this.wikilinkLookup })
  }

  // [[nome]] resolvido pelo mapa de notas que veio do servidor (WikilinkHelper), sem chamar a API.
  get wikilinkLookup() {
    return {
      resolve: (name) => resolveWikilink(name, this.wikilinksValue, this.folderValue),
      missingTitle: (name) => this.missingNoteValue.replace("%{name}", name),
    }
  }

  get mode() {
    return this.element.dataset.mode
  }
}
