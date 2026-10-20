// Câmera do grafo: translação (x, y, em px da tela) e escala k sobre o mundo da simulação. Zoom pela roda, pela pinça e
// pelo + e -, sempre em volta de um ponto da tela; fit enquadra os nós com uma margem.
const MIN_ZOOM = 0.1
const MAX_ZOOM = 4
// Enquadrar um grafo de um ou dois nós não o põe do tamanho da tela.
const FIT_MAX_ZOOM = 2
const FIT_MARGIN = 16

export class Camera {
  x = 0
  y = 0
  k = 1
  width = 0
  height = 0

  resize(width, height) {
    this.width = width
    this.height = height
  }

  toWorld(px, py) {
    return [ (px - this.x) / this.k, (py - this.y) / this.k ]
  }

  zoomAt(px, py, factor) {
    const [ wx, wy ] = this.toWorld(px, py)
    this.k = clamp(this.k * factor, MIN_ZOOM, MAX_ZOOM)
    this.x = px - wx * this.k
    this.y = py - wy * this.k
  }

  zoomCenter(factor) {
    this.zoomAt(this.width / 2, this.height / 2, factor)
  }

  panBy(dx, dy) {
    this.x += dx
    this.y += dy
  }

  fit(nodes) {
    const { minX, minY, maxX, maxY } = boundsOf(nodes)
    const room = (size) => Math.max(size - 2 * FIT_MARGIN, 1)
    this.k = clamp(Math.min(room(this.width) / (maxX - minX || 1), room(this.height) / (maxY - minY || 1)), MIN_ZOOM, FIT_MAX_ZOOM)
    this.centerOn((minX + maxX) / 2, (minY + maxY) / 2)
  }

  centerOn(wx, wy) {
    this.x = this.width / 2 - wx * this.k
    this.y = this.height / 2 - wy * this.k
  }

  shows(node) {
    const [ px, py ] = [ node.x * this.k + this.x, node.y * this.k + this.y ]
    return px >= 0 && py >= 0 && px <= this.width && py <= this.height
  }

  apply(context, ratio) {
    context.setTransform(ratio * this.k, 0, 0, ratio * this.k, ratio * this.x, ratio * this.y)
  }
}

// O retângulo dos nós com o raio de cada um.
function boundsOf(nodes) {
  const bounds = { minX: Infinity, minY: Infinity, maxX: -Infinity, maxY: -Infinity }
  for (const { x, y, radius } of nodes) {
    bounds.minX = Math.min(bounds.minX, x - radius)
    bounds.minY = Math.min(bounds.minY, y - radius)
    bounds.maxX = Math.max(bounds.maxX, x + radius)
    bounds.maxY = Math.max(bounds.maxY, y + radius)
  }
  return bounds
}

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max)
}
