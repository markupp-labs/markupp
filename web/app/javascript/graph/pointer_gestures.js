// Ponteiro no canvas do grafo, como no graph view do Obsidian: arrastar um nó o leva junto (a simulação reaquece),
// arrastar o fundo move a vista, a roda e a pinça (dois dedos) dão zoom em volta do ponteiro, tocar num nó sem arrastar
// abre a nota, e passar por cima (mouse ou caneta) destaca o nó e as vizinhas. A cena (scene.js) recebe cada gesto.
// Sem listener de scroll: a roda é do próprio canvas e fica nele (preventDefault), sem rolar a página.
const TAP_SLOP = 4
const WHEEL_SPEED = 0.0015

export function attachPointerGestures(canvas, scene) {
  const gestures = new PointerGestures(canvas, scene)
  const listeners = {
    pointerdown: (event) => gestures.down(event),
    pointermove: (event) => gestures.move(event),
    pointerup: (event) => gestures.up(event),
    pointercancel: (event) => gestures.up(event, { cancelled: true }),
    pointerleave: () => scene.hover(null),
    wheel: (event) => gestures.wheel(event)
  }
  Object.entries(listeners).forEach(([ type, listener ]) => canvas.addEventListener(type, listener, { passive: false }))
  return () => Object.entries(listeners).forEach(([ type, listener ]) => canvas.removeEventListener(type, listener))
}

class PointerGestures {
  pointers = new Map()

  constructor(canvas, scene) {
    this.canvas = canvas
    this.scene = scene
  }

  down(event) {
    this.canvas.setPointerCapture(event.pointerId)
    this.pointers.set(event.pointerId, pointOf(event))
    if (this.pointers.size > 1) return this.startPinch()

    const node = this.scene.nodeAt(...pointOf(event))
    this.press = { node, start: pointOf(event), last: pointOf(event), moved: false }
    if (node) this.scene.grab(node)
  }

  move(event) {
    const point = pointOf(event)
    if (!this.pointers.has(event.pointerId)) return this.hoverAt(event, point)

    this.pointers.set(event.pointerId, point)
    if (this.pinch) return this.pinchTo()
    if (this.press) this.dragTo(point)
  }

  up(event, { cancelled = false } = {}) {
    this.pointers.delete(event.pointerId)
    const press = this.press
    this.press = null
    this.pinch = null
    if (!press?.node) return

    this.scene.release(press.node)
    if (!press.moved && !cancelled) this.scene.open(press.node)
  }

  wheel(event) {
    event.preventDefault()
    this.scene.zoomAt(event.offsetX, event.offsetY, Math.exp(-event.deltaY * WHEEL_SPEED))
  }

  hoverAt(event, point) {
    if (event.pointerType != "touch") this.scene.hover(this.scene.nodeAt(...point))
  }

  dragTo(point) {
    const press = this.press
    press.moved ||= Math.hypot(point[0] - press.start[0], point[1] - press.start[1]) > TAP_SLOP
    if (press.node) this.scene.drag(press.node, ...point)
    else this.scene.panBy(point[0] - press.last[0], point[1] - press.last[1])
    press.last = point
  }

  // Dois dedos: o nó que o primeiro pegou é solto, e a distância entre eles vira o zoom.
  startPinch() {
    if (this.press?.node) this.scene.release(this.press.node)
    this.press = null
    this.pinch = { distance: spread(this.pointers) }
  }

  pinchTo() {
    const distance = spread(this.pointers)
    const [ a, b ] = [ ...this.pointers.values() ]
    this.scene.zoomAt((a[0] + b[0]) / 2, (a[1] + b[1]) / 2, distance / (this.pinch.distance || distance))
    this.pinch.distance = distance
  }
}

function pointOf(event) {
  return [ event.offsetX, event.offsetY ]
}

function spread(pointers) {
  const [ a, b ] = [ ...pointers.values() ]
  return Math.hypot(a[0] - b[0], a[1] - b[1])
}
