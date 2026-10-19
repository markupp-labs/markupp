import { Controller } from "@hotwired/stimulus"
import { renderMarkdown } from "editor/render_markdown"
import { resolveWikilink } from "editor/wikilinks"
import { scrollToHeading } from "editor/heading_ids"
import { embedContentLoader, fillEmbeds } from "editor/note_embeds"
import { addCodeTools, copyCode } from "editor/code_blocks"
import { editorScrollTopFor, previewScrollTopFor } from "editor/scroll_sync"
import { EditorView } from "@codemirror/view"

// Modos da nota: edição (CodeMirror), leitura (markdown renderizado) e lado a lado (preview a cada tecla).
// O modo vai no data-mode do form (o CSS mostra e esconde os painéis) e no cookie note_mode (NoteModeHelper).
// Atalhos: Ctrl+E alterna edição e leitura, como no Obsidian; Ctrl+\ alterna o lado a lado, como o split do VS Code.
const COOKIE_MAX_AGE = 60 * 60 * 24 * 365
const SHORTCUTS = { e: "read", "\\": "split" }

export default class extends Controller {
  static targets = [ "source", "preview", "button", "codeTools", "embedSpinner" ]
  static values = { wikilinks: Object, folder: String, missingNote: String, copied: String, copyFailed: String }

  connect() {
    this.render()
  }

  disconnect() {
    cancelAnimationFrame(this.alignFrame)
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
    // Só a troca anima o painel que entra (motion.css); abrir a nota já mostra o modo parado.
    this.element.dataset.modeSwitched = ""
    this.element.dataset.mode = mode
    this.buttonTargets.forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.noteModeModeParam == mode)))
    document.cookie = `note_mode=${mode}; path=/; max-age=${COOKIE_MAX_AGE}; samesite=lax`
    this.render()
    if (mode != "read") this.element.querySelector(".cm-content")?.focus()
  }

  // Em edição o preview fica escondido e não precisa ser desenhado a cada tecla.
  render() {
    if (this.mode == "edit") return

    const env = { wikilinks: this.wikilinkLookup }
    const addTools = (root) => addCodeTools(root, this.codeToolsTarget)
    this.previewTarget.innerHTML = renderMarkdown(this.sourceTarget.value, env)
    addTools(this.previewTarget)
    fillEmbeds(this.previewTarget, this.embedContent, env, { spinner: this.embedSpinnerTarget, afterFill: addTools })
    if (this.mode == "split") this.alignPreview()
  }

  // [[nota#Título]] abre a nota com #slug na url. O título só existe depois do render, e a rolagem do Turbo procura o
  // id sem o prefixo user-content- (vai ao topo); por isso a rolagem vem no turbo:load, que chega depois da dele.
  scrollToHash() {
    if (this.mode != "edit") scrollToHeading(this.previewTarget, location.hash)
  }

  // Link só de âncora no preview ([[#Título]] ou um <a href="#x"> do markdown): rola aqui, sem visita do Turbo,
  // e deixa o hash na url para quem copiar o endereço.
  followAnchor(event) {
    const hash = event.target.closest?.(".note-preview a[href^='#']")?.getAttribute("href")
    if (!hash || !scrollToHeading(this.previewTarget, hash)) return

    event.preventDefault()
    history.replaceState(history.state, "", hash)
  }

  copyCode({ currentTarget }) {
    copyCode(currentTarget, { copied: this.copiedValue, failed: this.copyFailedValue })
  }

  // Lado a lado, mão dupla como no VS Code: o scroll de um painel (o scroller do CodeMirror ou o preview, pego na
  // captura, scroll não sobe) leva o outro junto, no máximo um alinhamento por quadro.
  followScroll({ target }) {
    if (this.mode != "split" || !this.scrollPanes.includes(target)) return
    // Trava de origem: o scroll que o próprio alinhamento causou no outro painel não volta, senão um rola o outro em laço.
    if (target == this.alignedPane) return (this.alignedPane = null)

    cancelAnimationFrame(this.alignFrame)
    this.alignFrame = requestAnimationFrame(() => {
      this.alignFrame = null
      target == this.previewTarget ? this.alignEditor() : this.alignPreview()
    })
  }

  alignPreview() {
    const view = this.editorView
    if (view) this.scrollPane(this.previewTarget, previewScrollTopFor(view, this.previewTarget))
  }

  alignEditor() {
    const view = this.editorView
    if (view) this.scrollPane(view.scrollDOM, editorScrollTopFor(view, this.previewTarget))
  }

  // Salto direto, sem rolagem suave: acompanha o outro painel quadro a quadro e não anima nada (prefers-reduced-motion).
  // Só uma rolagem que de fato mexeu dispara scroll; é ela que a trava espera.
  scrollPane(pane, top) {
    const before = pane.scrollTop
    pane.scrollTo({ top, behavior: "instant" })
    this.alignedPane = pane.scrollTop == before ? null : pane
  }

  get scrollPanes() {
    return [ this.previewTarget, this.editorView?.scrollDOM ]
  }

  get editorView() {
    return EditorView.findFromDOM(this.element.querySelector(".cm-editor"))
  }

  // [[nome]] resolvido pelo mapa de notas que veio do servidor (WikilinkHelper), sem chamar a API.
  // A troca por função deixa o nome literal, sem os padrões $& e $' do replace.
  get wikilinkLookup() {
    return {
      resolve: (name) => resolveWikilink(name, this.wikilinksValue, this.folderValue),
      missingTitle: (name) => this.missingNoteValue.replace("%{name}", () => name),
    }
  }

  get embedContent() {
    this.embedLoader ??= embedContentLoader({
      noteUrls: Object.values(this.wikilinksValue), ownUrl: location.pathname, ownContent: () => this.sourceTarget.value,
    })
    return this.embedLoader
  }

  get mode() {
    return this.element.dataset.mode
  }
}
