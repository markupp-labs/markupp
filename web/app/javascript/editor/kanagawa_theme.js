import { EditorView } from "@codemirror/view"
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language"
import { Tag, styleTags, tags } from "@lezer/highlight"

// O markdown marca InlineCode e o texto do bloco cercado com a mesma tag (monospace), e o fundo pintado no
// bloco sai trecho a trecho, serrilhado. A tag própria deixa o fundo só no código inline.
const inlineCode = Tag.define(tags.monospace)
export const inlineCodeTag = { props: [ styleTags({ InlineCode: inlineCode }) ] }

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
  { tag: inlineCode, backgroundColor: "var(--surface-sunken)", borderRadius: "0.375rem" },
  { tag: [ tags.link, tags.url ], color: "var(--accent)" },
  { tag: [ tags.processingInstruction, tags.quote ], color: "var(--ink-muted)" },
]

// Código dentro dos blocos cercados (editor/code_languages.js), nas cores --code-* da leitura (hljs-* no application.css).
const codeHighlight = [
  { tag: tags.keyword, color: "var(--code-keyword)" },
  { tag: [ tags.string, tags.special(tags.string), tags.regexp, tags.inserted ], color: "var(--code-string)" },
  { tag: [ tags.number, tags.bool, tags.null ], color: "var(--code-number)" },
  { tag: [ tags.function(tags.variableName), tags.function(tags.propertyName), tags.tagName ], color: "var(--code-function)" },
  { tag: [ tags.typeName, tags.className, tags.namespace ], color: "var(--code-type)" },
  { tag: [ tags.propertyName, tags.attributeName, tags.special(tags.variableName) ], color: "var(--code-identifier)" },
  { tag: [ tags.atom, tags.self, tags.constant(tags.variableName) ], color: "var(--code-constant)" },
  { tag: tags.standard(tags.variableName), color: "var(--code-builtin)" },
  { tag: [ tags.meta, tags.deleted ], color: "var(--code-special)" },
  { tag: tags.comment, color: "var(--ink-muted)", fontStyle: "italic" },
]

// Styled, como o source do Obsidian: as mesmas marcas à vista, com títulos maiores.
const headingSizes = [
  { tag: tags.heading1, fontSize: "1.5em" },
  { tag: tags.heading2, fontSize: "1.3em" },
  { tag: tags.heading3, fontSize: "1.15em" },
]

const HIGHLIGHTS = {
  plain: HighlightStyle.define([ ...commonHighlight, ...codeHighlight ]),
  styled: HighlightStyle.define([ ...commonHighlight, ...codeHighlight, ...headingSizes ]),
}

// style vem do data-editor-style do html (cookie lido no servidor); valor desconhecido cai no plain.
export function kanagawaTheme(style) {
  return [ editorTheme, syntaxHighlighting(HIGHLIGHTS[style] || HIGHLIGHTS.plain) ]
}
