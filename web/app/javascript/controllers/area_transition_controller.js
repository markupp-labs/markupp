import { Controller } from "@hotwired/stimulus"

// Áreas em ordem fixa, como os workspaces 1, 2 e 3 do Hyprland. As telas públicas ficam fora dela.
const AREA_ORDER = [ "notes", "tenants", "settings" ]

// Troca de área (o data-area do body) como a troca de workspace do Hyprland. O Turbo faz toda visita por view
// transition (meta view-transition no head), também pelo voltar e avançar do navegador; o motion.css só anima com o
// data-area-moved no <html>, que este controller põe quando a página nova é de outra área e tira quando é da mesma
// (nota para nota fica instantânea). Ir a uma área de número maior é "forward"; a uma menor, "back", que desliza para
// o outro lado. Entrar ou sair das telas públicas, fora da ordem, é sempre "forward". O turbo:before-render vem depois
// da captura da página velha e antes da nova.
export default class extends Controller {
  mark({ detail: { newBody } }) {
    const from = AREA_ORDER.indexOf(this.element.dataset.area), to = AREA_ORDER.indexOf(newBody.dataset.area)
    if (newBody.dataset.area == this.element.dataset.area) return document.documentElement.removeAttribute("data-area-moved")

    document.documentElement.dataset.areaMoved = Math.min(from, to) >= 0 && to < from ? "back" : "forward"
  }
}
