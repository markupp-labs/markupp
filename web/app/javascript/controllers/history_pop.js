// O popstate antes do Turbo: application.js importa este módulo antes dele, então este ouvinte vem primeiro. Repete o
// popstate como history:pop, cancelável; quem o cancela trata o passo na própria página e o Turbo não o vê.
addEventListener("popstate", (event) => {
  const pop = new CustomEvent("history:pop", { cancelable: true, detail: { state: event.state } })
  if (!dispatchEvent(pop)) event.stopImmediatePropagation()
})
