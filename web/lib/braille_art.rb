# Converte uma imagem em tons de cinza em texto braille: cada caractere cobre 2x4 pixels e
# cada pixel mais escuro que o limiar acende o seu ponto. Usado pelo script/mascot.rb.
module BrailleArt
  BLANK = 0x2800
  # [coluna, linha] de cada ponto da célula, na ordem dos bits do Unicode (pontos 1 a 8).
  DOTS = [ [ 0, 0 ], [ 0, 1 ], [ 0, 2 ], [ 1, 0 ], [ 1, 1 ], [ 1, 2 ], [ 0, 3 ], [ 1, 3 ] ].freeze

  # pixels: linhas de valores 0 (preto) a 255 (branco), largura múltipla de 2 e altura de 4.
  def self.from_gray(pixels, threshold:)
    check_size(pixels)
    pixels.each_slice(4).map { |band| line(band, threshold) }.join("\n")
  end

  def self.line(band, threshold)
    (0...band.first.size).step(2).map { |left| cell(band, left, threshold) }.join
  end

  def self.cell(band, left, threshold)
    bits = DOTS.each_with_index.sum { |(column, row), bit| band[row][left + column] < threshold ? 1 << bit : 0 }
    (BLANK + bits).chr(Encoding::UTF_8)
  end

  def self.check_size(pixels)
    width, height = pixels.first.size, pixels.size
    return if (width % 2).zero? && (height % 4).zero?

    raise ArgumentError, "Image of #{width}x#{height} pixels, expected a size multiple of 2x4"
  end

  private_class_method :line, :cell, :check_size
end
