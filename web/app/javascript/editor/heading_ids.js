// Id de cada título da leitura, para o [[nota#Título]] rolar até ele. Slug estável: minúsculas, só letras,
// números, espaço e hífen, espaços viram hífen; título repetido ganha -1, -2, como no GitHub. O DOMPurify
// prefixa o id com user-content- (SANITIZE_NAMED_PROPS), e quem rola busca por esse prefixo (scrollToHeading).
export const headingSlug = (text) => text.trim().toLowerCase().replace(/[^\p{L}\p{N}\s-]/gu, "").replace(/\s+/g, "-")

const HEADING_ID_PREFIX = "user-content-"

// Regra core do markdown-it.
export function headingIds({ tokens }) {
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
export function scrollToHeading(root, hash) {
  const slug = hash && decodeURIComponent(hash.replace(/^#/, ""))
  const heading = slug && root.querySelector(`[id="${CSS.escape(HEADING_ID_PREFIX + slug)}"]`)
  if (!heading) return false

  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches
  heading.scrollIntoView({ behavior: reduced ? "instant" : "smooth", block: "start" })
  return true
}
