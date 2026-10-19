import { renderMarkdown } from "editor/render_markdown"

// Corpo de cada ![[nota]] do preview (editor/wikilinks.js deixa o bloco vazio): a nota renderizada pelo mesmo
// renderMarkdown, logo pelo mesmo DOMPurify, marcada como embutida (sem embed dentro, sem ids, sem linhas do editor).
// Sem conteúdo (erro, sessão vencida, nota apagada) o bloco fica só com o link para abrir a nota. afterFill recebe
// o corpo preenchido, para o que o preview ganha depois do render (as ferramentas dos blocos de código).
export function fillEmbeds(root, contentFor, env, afterFill) {
  root.querySelectorAll(".note-embed[data-embed-url]").forEach(async (embed) => {
    const body = embed.querySelector(".note-embed-body")
    markLoading(body)
    const content = await contentFor(embed.dataset.embedUrl)
    body.removeAttribute("aria-busy")
    body.innerHTML = content == null ? "" : renderMarkdown(content, { ...env, embedded: true })
    if (content != null) afterFill(body)
  })
}

// Enquanto o conteúdo não chega o corpo fica aria-busy com o indicador de carregamento (busy_spinner_controller.js),
// que só aparece se a espera passar de 300ms; o rótulo é o padrão do controller.
function markLoading(body) {
  body.setAttribute("aria-busy", "true")
  body.innerHTML = '<span class="busy-spinner" role="status" data-controller="busy-spinner"></span>'
}

// url -> conteúdo. A própria nota vem da página (o texto atual, até o não salvo); as outras do GET /notes/:id.json,
// uma vez por página: no lado a lado o preview redesenha a cada tecla. Só urls de notas da árvore: um
// data-embed-url escrito à mão no HTML do markdown não faz o navegador buscar outro endereço.
// ponytail: cache até recarregar a página; nota embutida editada em outra aba só atualiza no reload.
export function embedContentLoader({ noteUrls, ownUrl, ownContent }) {
  const known = new Set(noteUrls)
  const cache = new Map()
  return (url) => {
    if (url == ownUrl) return Promise.resolve(ownContent())
    if (!known.has(url)) return Promise.resolve(null)
    if (!cache.has(url)) cache.set(url, fetchNoteContent(url))
    return cache.get(url)
  }
}

// Sessão vencida ou tenant perdido respondem com redirect para uma página HTML: só json conta.
async function fetchNoteContent(url) {
  try {
    const response = await fetch(`${url}.json`, { headers: { Accept: "application/json" } })
    if (!response.ok || !response.headers.get("content-type")?.includes("json")) return null
    return (await response.json()).content
  } catch {
    return null
  }
}
