require "test_helper"

# As páginas de public/ respondem quando o próprio painel não consegue: autocontidas, no visual dele, sem nada de fora.
class StaticErrorPagesTest < ActionDispatch::IntegrationTest
  PAGES = { "400.html" => 400, "404.html" => 404, "406-unsupported-browser.html" => 406, "422.html" => 422, "500.html" => 500 }.freeze

  PAGES.each do |file, code|
    test "#{file} carries the panel look with no outside resource" do
      html = Rails.public_path.join(file).read
      page = Nokogiri::HTML5(html)

      assert_match(/\A#{code}: \S/, page.at_css("main h1")&.text.to_s)
      assert_includes page.at_css("pre[data-mascot=large]")&.text.to_s, MascotHelper::MASCOT_TEXT_BY_SIZE.fetch(:large)
      assert_includes html, 'url("/FiraCode-VF.woff2")'
      assert_includes html, "prefers-color-scheme: dark"
      assert_includes html, ':root[data-theme="dark"]'
      assert_match(/document\.cookie\.match\(.*theme=\(light\|dark\)/, page.at_css("head script:not([src])")&.text.to_s,
                   "expected #{file} to take the theme from the cookie, as the panel does")
      assert_empty page.css("script[src], link[href], img, iframe"), "expected #{file} self-contained, without loaded resources"
      assert_no_match(%r{https?://|url\((?!"/FiraCode-VF\.woff2")}, html, "expected #{file} to reference only the panel font")
    end
  end

  test "the font of the static pages is served at the root" do
    get "/FiraCode-VF.woff2"

    assert_response :success
    assert_equal "font/woff2", response.media_type
  end
end
