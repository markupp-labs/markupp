require "test_helper"

class MascotHelperTest < ActionView::TestCase
  test "the mascot is braille text hidden from screen readers" do
    pre = Nokogiri::HTML5.fragment(mascot(:large)).at_css("pre")

    assert_equal "large", pre["data-mascot"]
    assert_equal "true", pre["aria-hidden"]
    assert_match(/\A[⠀-⣿\n]+\z/, pre.text)
  end

  test "the small mascot is shorter than the large one" do
    small = Nokogiri::HTML5.fragment(mascot(:small)).at_css("pre").text
    large = Nokogiri::HTML5.fragment(mascot(:large)).at_css("pre").text

    assert_operator small.lines.size, :<, large.lines.size
  end

  # O logo é quadrado e cada caractere ocupa 0,6 x 1 em (linha de altura 1): largura em em / linhas perto de 1.
  # As linhas em branco das bordas são cortadas, daí a folga.
  test "the large mascot keeps the square shape of the logo" do
    lines = Nokogiri::HTML5.fragment(mascot(:large)).at_css("pre").text.lines.map(&:chomp)
    ratio = lines.map(&:size).max * 0.6 / lines.size

    assert_in_delta 1.0, ratio, 0.05, "expected a width/height near 1, got #{ratio.round(3)} (#{lines.map(&:size).max}x#{lines.size} characters)"
  end

  # 12x6 caracteres viravam uma mancha: o rosto precisa de pelo menos 20x8 para os olhos e a franja aparecerem.
  test "the small mascot has room for a recognizable face" do
    lines = Nokogiri::HTML5.fragment(mascot(:small)).at_css("pre").text.lines.map(&:chomp)

    assert_operator lines.size, :>=, 8
    assert_operator lines.map(&:size).max, :>=, 20
  end

  test "the mascot takes extra classes" do
    pre = Nokogiri::HTML5.fragment(mascot(:small, class: "hidden")).at_css("pre")

    assert_includes pre["class"].split, "hidden"
  end

  test "an unknown size names itself and the sizes there are" do
    error = assert_raises(ArgumentError) { mascot(:huge) }

    assert_match(/:huge.*:large.*:small/, error.message)
  end
end
