import MarkdownIt from "markdown-it"
import DOMPurify from "dompurify"
import { embedRule, wikilinkRule } from "editor/wikilinks"
import { highlightCode } from "editor/highlight_code"
import { headingIds } from "editor/heading_ids"

// Markdown para HTML na leitura e no lado a lado. HTML dentro do markdown é aceito, como no Obsidian,
// mas tudo passa pelo DOMPurify antes de entrar na página: sem script, sem on*, sem javascript:.
const markdownIt = new MarkdownIt({ html: true, linkify: true, highlight: highlightCode })
markdownIt.inline.ruler.before("link", "wikilink", wikilinkRule)
markdownIt.inline.ruler.before("image", "note_embed", embedRule)
markdownIt.core.ruler.push("heading_ids", headingIds)
// Cada bloco leva a linha do markdown onde começa, para o lado a lado alinhar o preview ao editor (scroll_sync.js).
// Nota embutida (env.embedded) fica sem: as linhas dela não são as do editor.
markdownIt.core.ruler.push("source_line", ({ tokens, env }) => {
  if (env.embedded) return
  tokens.filter((token) => token.map && token.nesting >= 0).forEach((token) => token.attrSet("data-source-line", String(token.map[0])))
})
const purifier = DOMPurify()
// O preview mora dentro do form da nota: um input ou botão vindo do markdown seria enviado (ou submeteria) junto
// com ela, como um _method=delete. Controles de formulário não passam.
const FORM_CONTROLS = [ "form", "input", "button", "select", "textarea", "option", "optgroup", "datalist" ]

// Link para fora abre em outra aba, sem dar ao destino acesso a esta (rel noopener).
purifier.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName != "A" || !/^https?:\/\//i.test(node.getAttribute("href") ?? "")) return

  node.setAttribute("target", "_blank")
  node.setAttribute("rel", "noopener noreferrer")
})

// env.wikilinks resolve os [[wikilinks]] (editor/wikilinks.js); sem ele ficam como texto. env.embedded marca o
// render de uma nota embutida. SANITIZE_NAMED_PROPS
// prefixa todo id e name com user-content-: um título "Title" ou um <a name> do markdown não clobbera o DOM da página.
export function renderMarkdown(source, env = {}) {
  return purifier.sanitize(markdownIt.render(source, env), { FORBID_TAGS: FORM_CONTROLS, SANITIZE_NAMED_PROPS: true })
}
