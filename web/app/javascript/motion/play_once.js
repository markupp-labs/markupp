// Marca de movimento de uso único: o motion.css só anima a troca com a marca no holder. Ela entra junto com a troca
// e sai quando o movimento que a troca começou acaba; o mesmo conteúdo redesenhado depois (morph do Turbo, connect de
// controller, re-render) ou a alça arrastada não anima de novo. Sem movimento (reduced-motion) sai no ato. Trocas
// seguidas: só a última tira a marca, a anterior acaba interrompida.
const latestTurn = new WeakMap()

export async function playOnce(holder, attribute, change) {
  const turn = Symbol(attribute)
  latestTurn.set(holder, turn)
  holder.setAttribute(attribute, "")
  change()
  await Promise.allSettled(finiteMotions(holder).map((motion) => motion.finished))
  if (latestTurn.get(holder) == turn) holder.removeAttribute(attribute)
}

// O que gira sem fim (indicador de carregamento, cursor do CodeMirror) nunca acaba e não é da troca.
function finiteMotions(holder) {
  return holder.getAnimations({ subtree: true }).filter((motion) => motion.effect.getComputedTiming().endTime != Infinity)
}
