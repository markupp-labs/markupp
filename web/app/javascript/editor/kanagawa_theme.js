import { EditorView } from "@codemirror/view"
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language"
import { tags } from "@lezer/highlight"

// Cores só pelas variáveis CSS do tema (application.css), então vale para o claro e o escuro.
const editorTheme = EditorView.theme({
  "&": { flexGrow: "1", color: "var(--ink)", backgroundColor: "transparent" },
  "&.cm-focused": { outline: "none" },
  ".cm-scroller": { fontFamily: "inherit", lineHeight: "1.6" },
  // Área de escrita alta mesmo vazia: clicar abaixo do texto já escreve (nota em branco não some na tela).
  ".cm-content": { caretColor: "var(--accent)", minHeight: "50dvh" },
  ".cm-cursor": { borderLeftColor: "var(--accent)" },
  ".cm-wikilink": { color: "var(--accent)", textDecoration: "underline" },
})

// Como no VS Code: o markdown aparece como está escrito, tudo no mesmo tamanho, e a sintaxe só ganha cor e peso.
const markdownHighlight = HighlightStyle.define([
  { tag: tags.heading, color: "var(--accent)", fontWeight: "700" },
  { tag: tags.strong, fontWeight: "700" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.monospace, backgroundColor: "var(--surface-sunken)", borderRadius: "0.375rem" },
  { tag: [ tags.link, tags.url ], color: "var(--accent)" },
  { tag: [ tags.processingInstruction, tags.quote ], color: "var(--ink-muted)" },
])

export const kanagawaTheme = [ editorTheme, syntaxHighlighting(markdownHighlight) ]
