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

# columns x lines em caracteres; o pixel do braille é alto (célula ~0.6 x 1), por isso a altura é esticada.
# crop recorta o rosto para a versão pequena; threshold mais alto acende mais pontos onde o traço é fino.
VERSIONS = {
  "large" => { columns: 60, lines: 32, threshold: 160, crop: nil },
  "small" => { columns: 12, lines: 6, threshold: 170, crop: "220x220+50+10" }
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
