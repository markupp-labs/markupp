// [[nome]], [[pasta/nome]], [[nome|texto]] e [[nome#título]] na leitura, como no Obsidian. A regra só monta os
// tokens; quem resolve o nome é o env.wikilinks de cada render ({ resolve(target), missingTitle(name) }).
const WIKILINK = /^\[\[([^[\]|\n]+)(?:\|([^[\]\n]+))?\]\]/

export function wikilinkRule(state, silent) {
  const match = state.src.startsWith("[[", state.pos) && WIKILINK.exec(state.src.slice(state.pos))
  const name = match && match[1].split("#")[0].trim()
  if (!name || !state.env.wikilinks) return false

  if (!silent) pushWikilink(state, name, (match[2] ?? match[1]).trim())
  state.pos += match[0].length
  return true
}

// Nota achada vira <a> interno; nome sem nota vira <span> marcado, com o motivo no title (mouse) e num
// texto sr-only (leitor de tela, que não lê o title de um span com confiança).
function pushWikilink(state, name, text) {
  const url = state.env.wikilinks.resolve(name)
  if (url) return pushWrapped(state, "link", "a", [ [ "href", url ], [ "class", "wikilink" ] ], text)

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
