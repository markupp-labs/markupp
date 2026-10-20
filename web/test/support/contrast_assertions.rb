# Contraste WCAG AA de um elemento contra o fundo dele, para os testes de sistema: 4.5:1 para texto comum, 3:1 para
# texto grande.
module ContrastAssertions
  # Cor de qualquer formato (light-dark, color-mix, oklab) vira rgba pelo canvas; o fundo é o do primeiro
  # ancestral com fundo, e o texto translúcido é misturado sobre ele antes da conta.
  CONTRAST_SCRIPT = <<~JS.freeze
    const [element, pseudo] = arguments
    const context = document.createElement("canvas").getContext("2d")
    const rgba = (color) => { context.clearRect(0, 0, 1, 1); context.fillStyle = color; context.fillRect(0, 0, 1, 1); return [ ...context.getImageData(0, 0, 1, 1).data ] }
    let node = element, back = rgba(getComputedStyle(node).backgroundColor)
    while (back[3] == 0 && node.parentElement) { node = node.parentElement; back = rgba(getComputedStyle(node).backgroundColor) }
    const front = rgba(getComputedStyle(element, pseudo).color)
    const blended = front.slice(0, 3).map((value, index) => value * front[3] / 255 + back[index] * (1 - front[3] / 255))
    const luminance = (rgb) => rgb.map((value) => { const c = value / 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4 })
                                  .reduce((sum, c, index) => sum + c * [ 0.2126, 0.7152, 0.0722 ][index], 0)
    const [ light, dark ] = [ luminance(blended), luminance(back.slice(0, 3)) ].sort((a, b) => b - a)
    return (light + 0.05) / (dark + 0.05)
  JS

  def assert_readable(element, pseudo: nil, minimum: 4.5)
    ratio = evaluate_script("(function() { #{CONTRAST_SCRIPT} }).apply(null, arguments)", element, pseudo)
    assert_operator ratio, :>=, minimum, "contrast of #{element.tag_name} #{element.text.inspect}#{pseudo} is #{ratio.round(2)}:1, expected at least #{minimum}:1"
  end
end
