#!/usr/bin/env ruby
# Gera o mascote em braille a partir de docs/assets/logo.png. Rodado uma vez; o texto vai commitado.
#
#   cd web && ruby script/mascot.rb
#
# Precisa do ImageMagick 7 (comando magick) no PATH, só para ler o PNG e reduzir em tons de cinza, e do rsvg-convert
# (librsvg) para os PNGs do favicon.
# Escreve app/assets/mascot/large.txt (telas públicas e páginas de erro) e small.txt (marca da barra lateral), e o
# favicon a partir do small: public/icon.svg (segue o tema do sistema) e os PNGs no claro.
require_relative "../lib/braille_art"
require_relative "../lib/braille_svg"

WEB = File.expand_path("..", __dir__)
LOGO = File.join(WEB, "../docs/assets/logo.png")

# columns x lines em caracteres; o ponto do braille é 1.2 vezes mais largo que alto (célula 0.6 x 1 com 2x4 pontos),
# então um recorte quadrado pede columns = lines * 5 / 3 para não achatar o rosto.
# crop recorta cabeça e franja para a versão pequena; threshold mais alto acende mais pontos onde o traço é fino.
# No pequeno o limiar é baixo: só o cabelo e o contorno acendem, e o rosto fica vazado e legível a 4px.
VERSIONS = {
  "large" => { columns: 55, lines: 33, threshold: 160, crop: nil },
  "small" => { columns: 20, lines: 12, threshold: 100, crop: "220x220+50+30" }
}.freeze

def gray_pixels(columns:, lines:, crop:, **)
  width, height = columns * 2, lines * 4
  command = [ "magick", LOGO, "-background", "white", "-alpha", "remove", *([ "-crop", crop, "+repage" ] if crop),
              "-colorspace", "Gray", "-resize", "#{width}x#{height}!", "-depth", "8", "gray:-" ]
  bytes = IO.popen(command, "rb", &:read).bytes
  raise "magick exited with #{$?.exitstatus} for #{LOGO}, expected #{width * height} gray bytes" unless $?.success?

  bytes.each_slice(width).to_a
end

VERSIONS.each do |name, options|
  art = BrailleArt.from_gray(gray_pixels(**options), threshold: options[:threshold])
  path = File.join(WEB, "app/assets/mascot/#{name}.txt")
  File.write(path, art.lines.map(&:rstrip).reject { |line| line.delete("⠀").empty? }.join("\n") + "\n")
  puts "#{path}: #{art.lines.size} lines"
end

FAVICON_PNGS = { "favicon-32.png" => 32, "apple-touch-icon.png" => 180 }.freeze

svg_path = File.join(WEB, "public/icon.svg")
File.write(svg_path, BrailleSvg.from_lines(File.readlines(File.join(WEB, "app/assets/mascot/small.txt"), chomp: true)))
puts svg_path
FAVICON_PNGS.each do |name, size|
  png_path = File.join(WEB, "public", name)
  system("rsvg-convert", "-w", size.to_s, "-h", size.to_s, svg_path, "-o", png_path, exception: true)
  puts "#{png_path}: #{size}x#{size}"
end
