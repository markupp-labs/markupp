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

// Plain, como no VS Code: o markdown aparece como está escrito, tudo no mesmo tamanho, e a sintaxe só ganha cor e peso.
const commonHighlight = [
  { tag: tags.heading, color: "var(--accent)", fontWeight: "700" },
  { tag: tags.strong, fontWeight: "700" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.monospace, backgroundColor: "var(--surface-sunken)", borderRadius: "0.375rem" },
  { tag: [ tags.link, tags.url ], color: "var(--accent)" },
  { tag: [ tags.processingInstruction, tags.quote ], color: "var(--ink-muted)" },
]

// Styled, como o source do Obsidian: as mesmas marcas à vista, com títulos maiores.
const headingSizes = [
  { tag: tags.heading1, fontSize: "1.5em" },
  { tag: tags.heading2, fontSize: "1.3em" },
  { tag: tags.heading3, fontSize: "1.15em" },
]

const HIGHLIGHTS = {
  plain: HighlightStyle.define(commonHighlight),
  styled: HighlightStyle.define([ ...commonHighlight, ...headingSizes ]),
}

// style vem do data-editor-style do html (cookie lido no servidor); valor desconhecido cai no plain.
export function kanagawaTheme(style) {
  return [ editorTheme, syntaxHighlighting(HIGHLIGHTS[style] || HIGHLIGHTS.plain) ]
}
