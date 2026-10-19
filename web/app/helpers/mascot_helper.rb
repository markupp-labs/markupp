# Mascote do markupp em braille, gerado do logo por script/mascot.rb. É decorativo: some para o leitor de tela,
# e a página ao lado já diz o que importa (formulário, nome da marca, mensagem de erro).
module MascotHelper
  MASCOT_DIR = Rails.root.join("app/assets/mascot")
  MASCOT_SIZES = %i[large small].freeze
  MASCOT_TEXT_BY_SIZE = MASCOT_SIZES.index_with { |size| MASCOT_DIR.join("#{size}.txt").read.chomp }.freeze

  # class acrescenta classes (ex.: esconder no celular); a cor vem do texto em volta ou da classe passada.
  def mascot(size, class: nil)
    text = MASCOT_TEXT_BY_SIZE.fetch(size) do
      raise ArgumentError, "Unknown mascot size #{size.inspect}, expected one of #{MASCOT_SIZES.map(&:inspect).join(', ')}"
    end
    tag.pre(text, class: [ "mascot", "mascot-#{size}", binding.local_variable_get(:class) ], data: { mascot: size }, aria: { hidden: true })
  end
end
