import { Controller } from "@hotwired/stimulus"
import { EditorState } from "@codemirror/state"
import { EditorView, keymap } from "@codemirror/view"
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands"
import { markdown, markdownLanguage } from "@codemirror/lang-markdown"
import { wikilinkMarks } from "editor/wikilink_marks"
import { inlineCodeTag, kanagawaTheme } from "editor/kanagawa_theme"

// Monta o CodeMirror sobre o textarea, que segue como fonte do formulário: cada edição volta para ele
// e dispara input, como se o usuário tivesse digitado no textarea.
export default class extends Controller {
  static targets = [ "source" ]

  connect() {
    this.view = new EditorView({ doc: this.sourceTarget.value, extensions: this.extensions(), parent: this.sourceTarget.parentElement })
    this.sourceTarget.hidden = true
  }

  disconnect() {
    this.view.destroy()
    this.sourceTarget.hidden = false
  }

  // Textarea readonly (papel viewer) vira editor só de leitura.
  extensions() {
    const readOnly = this.sourceTarget.readOnly
    return [
      EditorState.readOnly.of(readOnly), EditorView.editable.of(!readOnly),
      history(), keymap.of([ ...defaultKeymap, ...historyKeymap ]), EditorView.lineWrapping,
      markdown({ base: markdownLanguage, extensions: inlineCodeTag }), wikilinkMarks, kanagawaTheme(document.documentElement.dataset.editorStyle),
      EditorView.updateListener.of((update) => this.syncSource(update)),
    ]
  }

  // Tab no título: foco no conteúdo com o cursor no fim, pronto para continuar escrevendo.
  // Com o editor escondido (modo leitura) o Tab segue a ordem normal da página.
  focusEnd(event) {
    if (!this.view.dom.checkVisibility()) return

    event.preventDefault()
    this.view.focus()
    this.view.dispatch({ selection: { anchor: this.view.state.doc.length }, scrollIntoView: true })
  }

  // Troca o começo do documento (from) por to, só se ele ainda começa com from; applied avisa quem pediu.
  replaceStart({ detail }) {
    if (!this.view.state.doc.toString().startsWith(detail.from)) return

    this.view.dispatch({ changes: { from: 0, to: detail.from.length, insert: detail.to } })
    detail.applied = true
  }

  syncSource(update) {
    if (!update.docChanged) return

    this.sourceTarget.value = update.state.doc.toString()
    this.sourceTarget.dispatchEvent(new Event("input", { bubbles: true }))
  }
}
