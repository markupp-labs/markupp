import { Controller } from "@hotwired/stimulus"
import { EditorView, keymap } from "@codemirror/view"
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands"
import { markdown, markdownLanguage } from "@codemirror/lang-markdown"
import { livePreview, wikilinks } from "editor/live_preview"
import { kanagawaTheme } from "editor/kanagawa_theme"

// Monta o CodeMirror sobre o textarea, que segue como fonte do formulário: cada edição volta para ele
// e dispara input, como se o usuário tivesse digitado no textarea.
export default class extends Controller {
  static targets = [ "source" ]

  connect() {
    this.view = new EditorView({ doc: this.sourceTarget.value, extensions: this.extensions(), parent: this.element })
    this.sourceTarget.hidden = true
  }

  disconnect() {
    this.view.destroy()
    this.sourceTarget.hidden = false
  }

  extensions() {
    return [
      history(), keymap.of([ ...defaultKeymap, ...historyKeymap ]), EditorView.lineWrapping,
      markdown({ base: markdownLanguage }), livePreview, wikilinks, kanagawaTheme,
      EditorView.updateListener.of((update) => this.syncSource(update)),
    ]
  }

  syncSource(update) {
    if (!update.docChanged) return

    this.sourceTarget.value = update.state.doc.toString()
    this.sourceTarget.dispatchEvent(new Event("input", { bubbles: true }))
  }
}
