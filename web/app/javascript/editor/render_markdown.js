import MarkdownIt from "markdown-it"
import DOMPurify from "dompurify"
import { embedRule, wikilinkRule } from "editor/wikilinks"
import { highlightCode } from "editor/highlight_code"
import { headingIds } from "editor/heading_ids"
import { withLineNumbers } from "editor/code_blocks"

// O preview mora dentro do form da nota: um input ou botão vindo do markdown seria enviado (ou submeteria) junto
// com ela, como um _method=delete. Controles de formulário não passam.
const FORM_CONTROLS = [ "form", "input", "button", "select", "textarea", "option", "optgroup", "datalist" ]

// Markdown para HTML na leitura e no lado a lado. HTML dentro do markdown é aceito, como no Obsidian, mas tudo
// passa pelo DOMPurify antes de entrar na página: sem script, sem on*, sem javascript:. O DOMPurify limpa num DOM,
// então recebe a window: a do navegador ou, no Node, a de um JSDOM (pacote markupp-markdown).
// env.wikilinks resolve os [[wikilinks]] (editor/wikilinks.js); sem ele ficam como texto. env.embedded marca o
// render de uma nota embutida. SANITIZE_NAMED_PROPS prefixa todo id e name com user-content-: um título "Title"
// ou um <a name> do markdown não clobbera o DOM da página.
export function createMarkdownRenderer(window) {
  const markdownIt = buildMarkdownIt()
  const purifier = buildPurifier(window)
  return (source, env = {}) => purifier.sanitize(markdownIt.render(source, env), { FORBID_TAGS: FORM_CONTROLS, SANITIZE_NAMED_PROPS: true })
}

let browserRenderer

// O render do painel, com a window da página, montado no primeiro uso.
export function renderMarkdown(source, env = {}) {
  browserRenderer ??= createMarkdownRenderer(globalThis.window)
  return browserRenderer(source, env)
}

function buildMarkdownIt() {
  const markdownIt = new MarkdownIt({ html: true, linkify: true, highlight: highlightCode })
  markdownIt.inline.ruler.before("link", "wikilink", wikilinkRule)
  markdownIt.inline.ruler.before("image", "note_embed", embedRule)
  markdownIt.core.ruler.push("heading_ids", headingIds)
  markdownIt.core.ruler.push("source_line", sourceLines)
  markdownIt.renderer.rules.fence = withLineNumbers(markdownIt.renderer.rules.fence)
  return markdownIt
}

// Cada bloco leva a linha do markdown onde começa, para o lado a lado alinhar o preview ao editor (scroll_sync.js).
// Nota embutida (env.embedded) fica sem: as linhas dela não são as do editor.
function sourceLines({ tokens, env }) {
  if (env.embedded) return
  tokens.filter((token) => token.map && token.nesting >= 0).forEach((token) => token.attrSet("data-source-line", String(token.map[0])))
}

// Sem DOM o DOMPurify fica isSupported false e devolve o html sem limpar: aí não há render.
function buildPurifier(window) {
  const purifier = DOMPurify(window)
  if (!purifier.isSupported) {
    throw new TypeError(`createMarkdownRenderer espera uma window com document (a do navegador ou new JSDOM("").window), recebeu ${describeWindow(window)}`)
  }

  purifier.addHook("afterSanitizeAttributes", openExternalInNewTab)
  return purifier
}

const describeWindow = (window) => (window == null ? String(window) : `um objeto com as chaves [${Object.keys(window).slice(0, 5).join(", ")}]`)

// Link para fora abre em outra aba, sem dar ao destino acesso a esta (rel noopener).
function openExternalInNewTab(node) {
  if (node.tagName != "A" || !/^https?:\/\//i.test(node.getAttribute("href") ?? "")) return

  node.setAttribute("target", "_blank")
  node.setAttribute("rel", "noopener noreferrer")
}
