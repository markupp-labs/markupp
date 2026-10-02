import { EditorView } from "@codemirror/view"
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language"
import { tags } from "@lezer/highlight"

// Cores só pelas variáveis CSS do tema (application.css), então vale para o claro e o escuro.
const editorTheme = EditorView.theme({
  "&": { flexGrow: "1", color: "var(--ink)", backgroundColor: "transparent" },
  "&.cm-focused": { outline: "none" },
  ".cm-scroller": { fontFamily: "inherit", lineHeight: "1.6" },
  ".cm-content": { caretColor: "var(--accent)" },
  ".cm-cursor": { borderLeftColor: "var(--accent)" },
  ".cm-wikilink": { color: "var(--accent)", textDecoration: "underline" },
})

const markdownHighlight = HighlightStyle.define([
  { tag: tags.heading1, fontWeight: "700", fontSize: "1.5em" },
  { tag: tags.heading2, fontWeight: "700", fontSize: "1.3em" },
  { tag: tags.heading3, fontWeight: "700", fontSize: "1.15em" },
  { tag: [ tags.heading4, tags.heading5, tags.heading6, tags.strong ], fontWeight: "700" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.monospace, backgroundColor: "var(--surface-sunken)", borderRadius: "0.375rem" },
  { tag: [ tags.link, tags.url ], color: "var(--accent)" },
  { tag: [ tags.processingInstruction, tags.quote ], color: "var(--ink-muted)" },
])

export const kanagawaTheme = [ editorTheme, syntaxHighlighting(markdownHighlight) ]
