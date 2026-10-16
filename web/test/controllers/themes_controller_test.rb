require "test_helper"

class ThemesControllerTest < ActionDispatch::IntegrationTest
  test "update to light keeps a permanent cookie and goes back" do
    patch theme_path, params: { theme: "light" }, headers: { "HTTP_REFERER" => "http://www.example.com/somewhere" }

    assert_redirected_to "http://www.example.com/somewhere"
    assert_equal "light", cookies[:theme]
    assert_match(/expires=/i, response.headers["set-cookie"].to_s)
  end

  test "update to dark falls back to the root without a referer" do
    patch theme_path, params: { theme: "dark" }

    assert_redirected_to root_path
    assert_equal "dark", cookies[:theme]
  end

  test "update rejects an unknown theme" do
    patch theme_path, params: { theme: "neon" }

    assert_response :bad_request
    assert_match(/"neon".*light or dark/, response.body)
    assert_nil cookies[:theme]
  end

  test "layout marks the html with the theme from the cookie" do
    use_fake_api(notes: [])
    sign_in_as
    cookies[:theme] = "dark"

    get root_path

    assert_select "html[data-theme=dark]"
  end

  test "layout leaves the theme to the system without a cookie" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "html:not([data-theme])"
    assert_select "nav form.theme-to-light button", "Light theme"
    assert_select "nav form.theme-to-dark button", "Dark theme"
  end
end
