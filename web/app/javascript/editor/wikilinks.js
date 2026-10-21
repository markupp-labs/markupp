import { headingSlug } from "editor/heading_ids"

// [[nome]], [[pasta/nome]], [[nome|texto]], [[nome#título]] e [[#título]] na leitura, como no Obsidian. A regra só
// monta os tokens; quem resolve o nome é o env.wikilinks de cada render ({ resolve(target), missingTitle(name) }).
const WIKILINK = /^\[\[([^[\]|\n]+)(?:\|([^[\]\n]+))?\]\]/

export function wikilinkRule(state, silent) {
  const match = state.src.startsWith("[[", state.pos) && WIKILINK.exec(state.src.slice(state.pos))
  const link = match && parseWikilink(match)
  if (!link || !state.env.wikilinks) return false

  if (!silent) pushWikilink(state, link)
  state.pos += match[0].length
  return true
}

// ![[nome]]: a nota inteira embutida, como no Obsidian. Sai um bloco com o link para abrir a nota e o corpo vazio,
// que o editor/note_embeds.js preenche. Dentro de uma nota embutida vira link (profundidade 1, ciclo não trava);
// nota que não existe vira o aviso de link quebrado.
export function embedRule(state, silent) {
  const match = state.src.startsWith("![[", state.pos) && WIKILINK.exec(state.src.slice(state.pos + 1))
  const link = match && parseWikilink(match)
  if (!link || !state.env.wikilinks) return false

  if (!silent) pushEmbed(state, link)
  state.pos += match[0].length + 1
  return true
}

function pushEmbed(state, link) {
  const url = link.name && state.env.wikilinks.resolve(link.name)
  if (!url || state.env.embedded) return pushWikilink(state, link)

  state.push("note_embed_open", "span", 1).attrs = [ [ "class", "note-embed" ], [ "data-embed-url", url ] ]
  pushWrapped(state, "link", "a", [ [ "href", url ], [ "class", "wikilink note-embed-link" ] ], link.text)
  state.push("note_embed_body_open", "span", 1).attrs = [ [ "class", "note-embed-body" ] ]
  state.push("note_embed_body_close", "span", -1)
  state.push("note_embed_close", "span", -1)
}

// Nome, slug do título (vazio sem #) e texto; sem nome e sem título não é wikilink.
function parseWikilink(match) {
  const [ name, ...heading ] = match[1].split("#")
  const link = { name: name.trim(), anchor: headingSlug(heading.join("#")), text: (match[2] ?? match[1]).trim() }
  return (link.name || link.anchor) && link
}

// Nota achada vira <a> interno (com #slug do título, se houver); só o título rola na própria nota. Nome sem nota
// vira <span> marcado, com o motivo no title (mouse) e num texto sr-only (leitor de tela, que não lê o title de um
// span com confiança).
function pushWikilink(state, { name, anchor, text }) {
  const url = name ? state.env.wikilinks.resolve(name) : ""
  const hash = anchor && `#${encodeURIComponent(anchor)}`
  if (url != null) return pushWrapped(state, "link", "a", [ [ "href", `${url}${hash}` ], [ "class", "wikilink" ] ], text)

  const reason = state.env.wikilinks.missingTitle(name)
  state.push("wikilink_missing_open", "span", 1).attrs = [ [ "class", "wikilink-missing" ], [ "title", reason ] ]
  state.push("text", "", 0).content = text
  pushWrapped(state, "wikilink_reason", "span", [ [ "class", "sr-only" ] ], ` (${reason})`)
  state.push("wikilink_missing_close", "span", -1)
}

function pushWrapped(state, type, tag, attrs, text) {
  state.push(`${type}_open`, tag, 1).attrs = attrs
  state.push("text", "", 0).content = text
  state.push(`${type}_close`, tag, -1)
}

// Como no Obsidian: sem diferença de maiúsculas, o nome casa com o fim do caminho e, com o mesmo nome em pastas
// diferentes, ganha a nota da pasta atual, senão o caminho mais curto (empate em ordem alfabética).
export function resolveWikilink(name, targets, folder) {
  const wanted = name.replace(/\.md$/i, "").toLowerCase()
  const sameFolder = [ folder, wanted ].filter(Boolean).join("/").toLowerCase()
  const candidates = Object.keys(targets).filter((path) => {
    const lower = path.toLowerCase()
    return lower == wanted || lower.endsWith(`/${wanted}`)
  })
  const chosen = candidates.find((path) => path.toLowerCase() == sameFolder) ?? candidates.sort(shorterFirst)[0]
  return chosen && targets[chosen]
}

const shorterFirst = (a, b) => a.length - b.length || a.localeCompare(b)
