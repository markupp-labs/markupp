// Cores do canvas pelos tokens do tema (application.css, tree.css). Os tokens são light-dark(), que só resolvem na cor
// computada: cada um passa por um elemento de prova escondido. As comunidades usam os mesmos matizes do Explorer
// (ExplorerHelper#community_hue). O halo atrás dos nomes é o fundo do palco, para o nome ler por cima das ligações.
const TOKENS = { line: "--line", ink: "--ink", muted: "--ink-muted", accent: "--accent" }
const COMMUNITY_HUES = 8

export function readThemeColors(stage) {
  const probe = stage.appendChild(document.createElement("span"))
  probe.hidden = true
  const resolve = (token) => {
    probe.style.color = `var(${token})`
    return getComputedStyle(probe).color
  }
  const colors = Object.fromEntries(Object.entries(TOKENS).map(([ name, token ]) => [ name, resolve(token) ]))
  colors.communities = Array.from({ length: COMMUNITY_HUES }, (_, hue) => resolve(`--community-${hue}`))
  colors.halo = getComputedStyle(stage).backgroundColor
  probe.remove()
  return colors
}
