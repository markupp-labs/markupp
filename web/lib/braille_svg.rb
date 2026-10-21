# Braille em SVG: cada ponto aceso de cada caractere vira um círculo numa grade de pontos, sem depender de fonte.
# Serve ao favicon (script/mascot.rb), que um navegador desenha antes de qualquer CSS ou fonte do painel.
module BrailleSvg
  # Margem e distância entre pontos, em unidades do viewBox; o desenho começa em PADDING e o viewBox é quadrado.
  PADDING = 2
  STEP = 1
  # Quase encostando no vizinho: a 16px o desenho é a média de fundo e pontos, e ponto fino apagava o rosto.
  RADIUS = 0.48
  BLANK = 0x2800

  # Bit do caractere braille (Unicode) => [coluna, linha] do ponto na célula de 2x4.
  DOTS = { 0x01 => [ 0, 0 ], 0x02 => [ 0, 1 ], 0x04 => [ 0, 2 ], 0x40 => [ 0, 3 ],
           0x08 => [ 1, 0 ], 0x10 => [ 1, 1 ], 0x20 => [ 1, 2 ], 0x80 => [ 1, 3 ] }.freeze

  # Kanagawa do painel (application.css), invertido como o favicon do GitHub para contrastar com a aba: no navegador
  # claro o ícone é escuro e no escuro, claro. As duas versões usam o par mais contrastante da paleta (o surface do
  # wave e o do lotus, 14:1), só trocando fundo e pontos. Os PNGs, sem tema, saem como no navegador claro.
  STYLE = ".bg{fill:#1f1f28}.dot{fill:#f2ecbc}" \
          "@media (prefers-color-scheme: dark){.bg{fill:#f2ecbc}.dot{fill:#1f1f28}}".freeze

  def self.from_lines(lines)
    width = lines.map(&:length).max.to_i * 2
    height = lines.size * 4
    side = [ width, height ].max + PADDING * 2
    origin = [ (width + PADDING * 2 - side) / 2.0, (height + PADDING * 2 - side) / 2.0 ]
    svg(origin, side, lit_dots(lines).map { |column, row| circle(column, row) })
  end

  def self.lit_dots(lines)
    lines.each_with_index.flat_map do |line, line_index|
      line.each_char.with_index.flat_map { |char, char_index| cell_dots(char, char_index * 2, line_index * 4) }
    end
  end

  def self.cell_dots(char, left, top)
    bits = char.ord - BLANK
    return [] unless bits.between?(1, 0xff)

    DOTS.filter_map { |bit, (column, row)| [ left + column, top + row ] if bits.anybits?(bit) }
  end

  def self.circle(column, row)
    center = ->(index) { (PADDING + (index + 0.5) * STEP).round(2) }
    %(<circle class="dot" cx="#{center.(column)}" cy="#{center.(row)}" r="#{RADIUS}"/>)
  end

  def self.svg((left, top), side, circles)
    %(<svg xmlns="http://www.w3.org/2000/svg" viewBox="#{left} #{top} #{side} #{side}">) +
      %(<style>#{STYLE}</style><rect class="bg" x="#{left}" y="#{top}" width="#{side}" height="#{side}" rx="#{(side / 6.0).round(2)}"/>) +
      circles.join + "</svg>\n"
  end

  private_class_method :lit_dots, :cell_dots, :circle, :svg
end
