// Id de cada título da leitura, para o [[nota#Título]] rolar até ele. Slug estável: minúsculas, sem acento, só
// letras, números, espaço e hífen, espaços viram hífen; título repetido ganha -1, -2, como no GitHub. O DOMPurify
// prefixa o id com user-content- (SANITIZE_NAMED_PROPS), e quem rola busca por esse prefixo (scrollToHeading).
export const headingSlug = (text) => withoutAccents(text.trim().toLowerCase()).replace(/[^\p{L}\p{N}\s-]/gu, "").replace(/\s+/g, "-")

// "ção" vira "cao": separa a letra do acento (NFD) e tira as marcas.
const withoutAccents = (text) => text.normalize("NFD").replace(/\p{M}/gu, "")

const HEADING_ID_PREFIX = "user-content-"

// Regra core do markdown-it. Nota embutida não leva ids: repetiriam os da nota em volta.
export function headingIds({ tokens, env }) {
  if (env.embedded) return

  const seen = new Map()
  tokens.forEach((token, index) => {
    const slug = token.type == "heading_open" && headingSlug(tokens[index + 1].content)
    if (!slug) return

    const count = seen.get(slug) ?? 0
    seen.set(slug, count + 1)
    token.attrSet("id", count ? `${slug}-${count}` : slug)
  })
}

// Rola até o título do hash (#slug, já com o encode da url) dentro de root; sem animação para quem pede menos movimento.
// Compara sem acento: o link de antes do slug sem acento (#configuração-inicial) ainda acha o título.
export function scrollToHeading(root, hash) {
  const slug = hash && withoutAccents(decodedSlug(hash.replace(/^#/, "")))
  const heading = slug && root.querySelector(`[id="${CSS.escape(HEADING_ID_PREFIX + slug)}"]`)
  if (!heading) return false

  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches
  heading.scrollIntoView({ behavior: reduced ? "instant" : "smooth", block: "start" })
  return true
}

// Hash malformado (#%, #%E0) não decodifica: não aponta para título nenhum.
function decodedSlug(encoded) {
  try {
    return decodeURIComponent(encoded)
  } catch {
    return ""
  }
}
