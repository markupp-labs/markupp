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

  private
    def circles(svg) = Nokogiri::XML(svg).css("circle")

    # Posição na grade de pontos, a partir do centro em unidades do viewBox.
    def position(circle) = [ circle["cx"].to_f, circle["cy"].to_f ].map { ((it - BrailleSvg::PADDING) / BrailleSvg::STEP - 0.5).round }
end
