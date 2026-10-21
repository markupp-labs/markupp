require "test_helper"

class IconsHelperTest < ActionView::TestCase
  test "a decorative icon is the Phosphor svg hidden from screen readers" do
    svg = Nokogiri::HTML5.fragment(icon(:sun)).at_css("svg")

    assert_equal "sun", svg["data-icon"]
    assert_equal "true", svg["aria-hidden"]
    assert_nil svg["role"]
    assert svg.at_css("path[d]")
  end

  test "a labeled icon is an image with that name" do
    svg = Nokogiri::HTML5.fragment(icon(:moon, label: "Dark theme")).at_css("svg")

    assert_equal "img", svg["role"]
    assert_equal "Dark theme", svg["aria-label"]
    assert_nil svg["aria-hidden"]
  end

  test "the icon takes the size classes it is given" do
    svg = Nokogiri::HTML5.fragment(icon(:sun, class: "size-5")).at_css("svg")

    assert_equal "size-5 shrink-0", svg["class"]
  end

  test "the icon escapes the label" do
    svg = Nokogiri::HTML5.fragment(icon(:sun, label: %(a "b" <c>))).at_css("svg")

    assert_equal %(a "b" <c>), svg["aria-label"]
  end

  test "an icon that is not vendored names itself and where icons live" do
    error = assert_raises(ArgumentError) { icon(:rocket) }

    assert_match(/"rocket".*vendor\/phosphor\/rocket\.svg/, error.message)
  end
end
