import { forceCollide, forceLink, forceManyBody, forceSimulation, forceX, forceY } from "d3-force"

// O d3-force atrás de uma interface do painel: a simulação nunca roda pelo timer dela (stop), quem anda é o step, um
// tick por quadro do requestAnimationFrame da cena, ou o settle, todos de uma vez, com reduced-motion. Decai mais rápido
// que o padrão do d3 (0.0228, uns 300 ticks): assenta em uns 170 ticks, menos de 3s a 60 quadros.
const ALPHA_DECAY = 0.04
const LINK_DISTANCE = 36
const CHARGE = -70
const CHARGE_RANGE = 360
const GRAVITY = 0.05
const COLLIDE_GAP = 2
const DRAG_HEAT = 0.3

export function createForceLayout(nodes, links, radiusOf) {
  const simulation = forceSimulation(nodes)
    .force("link", forceLink(links).distance(LINK_DISTANCE))
    .force("charge", forceManyBody().strength(CHARGE).distanceMax(CHARGE_RANGE))
    .force("collide", forceCollide((node) => radiusOf(node) + COLLIDE_GAP))
    .force("x", forceX().strength(GRAVITY))
    .force("y", forceY().strength(GRAVITY))
    .alphaDecay(ALPHA_DECAY)
    .stop()
  return layoutOf(simulation)
}

function layoutOf(simulation) {
  return {
    // Um tick; true enquanto a simulação ainda não assentou.
    step() {
      simulation.tick()
      return simulation.alpha() > simulation.alphaMin()
    },
    settle() {
      simulation.tick(ticksToSettle(simulation))
    },
    // Arrastar um nó reaquece, e soltar deixa esfriar de novo.
    heat() {
      simulation.alphaTarget(DRAG_HEAT).alpha(Math.max(simulation.alpha(), DRAG_HEAT))
    },
    cool() {
      simulation.alphaTarget(0)
    },
    nearest(x, y, radius) {
      return simulation.find(x, y, radius)
    }
  }
}

function ticksToSettle(simulation) {
  return Math.ceil(Math.log(simulation.alphaMin() / simulation.alpha()) / Math.log(1 - simulation.alphaDecay()))
}
