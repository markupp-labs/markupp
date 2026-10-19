import { Controller } from "@hotwired/stimulus"

// Troca de área (notas, configurações, tenants, telas públicas: o data-area do body) como a troca de workspace do
// Hyprland. O Turbo faz toda visita por view transition (meta view-transition no head); o motion.css só anima com o
// data-area-moved no <html>, que este controller põe quando a página nova é de outra área e tira quando é da mesma
// (nota para nota fica instantânea). O turbo:before-render vem depois da captura da página velha e antes da nova.
export default class extends Controller {
  mark({ detail: { newBody } }) {
    document.documentElement.toggleAttribute("data-area-moved", newBody.dataset.area != this.element.dataset.area)
  }
}
