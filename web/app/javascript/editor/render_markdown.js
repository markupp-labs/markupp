import MarkdownIt from "markdown-it"
import DOMPurify from "dompurify"

// Markdown para HTML na leitura e no lado a lado. HTML dentro do markdown é aceito, como no Obsidian,
// mas tudo passa pelo DOMPurify antes de entrar na página: sem script, sem on*, sem javascript:.
const markdownIt = new MarkdownIt({ html: true, linkify: true })
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

export function renderMarkdown(source) {
  return purifier.sanitize(markdownIt.render(source), { FORBID_TAGS: FORM_CONTROLS })
}
