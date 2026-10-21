require "test_helper"

# Braille em SVG: cada ponto aceso vira um círculo numa grade de pontos, sem depender de fonte (favicon).
class BrailleSvgTest < ActiveSupport::TestCase
  test "the full cell lights its eight dots" do
    assert_equal 8, circles(BrailleSvg.from_lines([ "⣿" ])).size
  end

  test "each dot lands on its own column and row of the cell" do
    assert_equal [ [ 0, 0 ] ], circles(BrailleSvg.from_lines([ "⠁" ])).map { position(it) }
    assert_equal [ [ 1, 3 ] ], circles(BrailleSvg.from_lines([ "⢀" ])).map { position(it) }
  end

  test "lines stack their cells four dots apart" do
    assert_equal [ [ 0, 4 ] ], circles(BrailleSvg.from_lines([ "⠀", "⠁" ])).map { position(it) }
  end

  test "the icon follows the light and the dark theme of the system" do
    svg = BrailleSvg.from_lines([ "⣿" ])

    assert_includes svg, "prefers-color-scheme: dark"
    assert_match(/viewBox="[\d. -]+"/, svg)
  end

  # Como o favicon do GitHub: contrasta com a aba, então é escuro no navegador claro e claro no escuro.
  test "the icon is dark on a light browser and light on a dark one" do
    style = Nokogiri::XML(BrailleSvg.from_lines([ "⣿" ])).at("style").text
    light_browser, dark_browser = style.split("@media (prefers-color-scheme: dark)")

    assert_includes light_browser, ".bg{fill:#1f1f28}"
    assert_includes dark_browser, ".bg{fill:#f2ecbc}"
  end

  # A 16px o ícone é a mistura do fundo com os pontos: com pontos finos e ink médio o rosto sumia no navegador escuro.
  # As duas versões usam o par mais contrastante da paleta e pontos que cobrem boa parte da célula.
  test "both versions keep a strong contrast between the background and the dots" do
    style = Nokogiri::XML(BrailleSvg.from_lines([ "⣿" ])).at("style").text

    style.split("@media (prefers-color-scheme: dark)").each do |theme|
      background, dots = theme.scan(/fill:(#\h{6})/).flatten
      assert_operator contrast(background, dots), :>=, 12, "#{dots} on #{background} in #{theme}"
    end
    assert_operator BrailleSvg::RADIUS, :>=, 0.48
  end

  private
    def circles(svg) = Nokogiri::XML(svg).css("circle")

    def contrast(*colors)
      light, dark = colors.map { luminance(it) }.sort.reverse
      (light + 0.05) / (dark + 0.05)
    end

    def luminance(hex)
      channels = hex.delete("#").scan(/../).map { it.to_i(16) / 255.0 }
      linear = channels.map { it <= 0.03928 ? it / 12.92 : ((it + 0.055) / 1.055)**2.4 }
      [ 0.2126, 0.7152, 0.0722 ].zip(linear).sum { |weight, channel| weight * channel }
    end

    # Posição na grade de pontos, a partir do centro em unidades do viewBox.
    def position(circle) = [ circle["cx"].to_f, circle["cy"].to_f ].map { ((it - BrailleSvg::PADDING) / BrailleSvg::STEP - 0.5).round }
end
