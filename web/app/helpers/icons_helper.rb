# Ícones Phosphor (peso light) vendorizados em vendor/phosphor, desenhados inline para herdar a cor do texto.
module IconsHelper
  ICONS_DIR = Rails.root.join("vendor/phosphor")
  SVG_BY_NAME = {} # ponytail: cache sem limite, cabe porque só há os poucos ícones vendorizados

  # Sem label o ícone é decorativo e some para o leitor de tela; com label ele é a imagem que nomeia o botão.
  # class troca o tamanho padrão (size-4).
  def icon(name, label: nil, **html)
    attributes = { data: { icon: name }, class: [ html.fetch(:class, "size-4"), "shrink-0" ] }
    attributes.merge!(label ? { role: "img", aria: { label: } } : { aria: { hidden: true } })
    # O SVG é o arquivo oficial e os atributos passam pelo escape do tag.attributes.
    phosphor_svg(name).sub("<svg ", "<svg #{tag.attributes(attributes)} ").html_safe
  end

  private
    def phosphor_svg(name)
      path = ICONS_DIR.join("#{name}.svg")
      raise ArgumentError, "Unknown icon #{name.to_s.inspect}, expected a vendored file at #{path.relative_path_from(Rails.root)}" unless path.file?

      SVG_BY_NAME[path.to_s] ||= path.read.strip
    end
end
