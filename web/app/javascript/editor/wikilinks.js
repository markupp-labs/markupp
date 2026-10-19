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

// Nota achada vira <a> interno; nome sem nota vira <span> marcado, com o title explicando.
function pushWikilink(state, name, text) {
  const url = state.env.wikilinks.resolve(name)
  const [ type, tag, attrs ] = url ? [ "link", "a", [ [ "href", url ], [ "class", "wikilink" ] ] ]
    : [ "wikilink_missing", "span", [ [ "class", "wikilink-missing" ], [ "title", state.env.wikilinks.missingTitle(name) ] ] ]
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
