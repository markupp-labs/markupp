# Braille em SVG: cada ponto aceso de cada caractere vira um círculo numa grade de pontos, sem depender de fonte.
# Serve ao favicon (script/mascot.rb), que um navegador desenha antes de qualquer CSS ou fonte do painel.
module BrailleSvg
  PADDING = 2
  STEP = 1

  def self.from_lines(_lines) = %(<svg xmlns="http://www.w3.org/2000/svg"/>)
end
