#!/usr/bin/env ruby
# Gera o mascote em braille a partir de docs/assets/logo.png. Rodado uma vez; o texto vai commitado.
#
#   cd web && ruby script/mascot.rb
#
# Precisa do ImageMagick 7 (comando magick) no PATH, só para ler o PNG e reduzir em tons de cinza.
# Escreve app/assets/mascot/large.txt (telas públicas e páginas de erro) e small.txt (marca da barra lateral).
require_relative "../lib/braille_art"

WEB = File.expand_path("..", __dir__)
LOGO = File.join(WEB, "../docs/assets/logo.png")

# columns x lines em caracteres; o ponto do braille é 1.2 vezes mais largo que alto (célula 0.6 x 1 com 2x4 pontos),
# então um recorte quadrado pede columns = lines * 5 / 3 para não achatar o rosto.
# crop recorta cabeça e franja para a versão pequena; threshold mais alto acende mais pontos onde o traço é fino.
# No pequeno o limiar é baixo: só o cabelo e o contorno acendem, e o rosto fica vazado e legível a 4px.
VERSIONS = {
  "large" => { columns: 60, lines: 32, threshold: 160, crop: nil },
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
