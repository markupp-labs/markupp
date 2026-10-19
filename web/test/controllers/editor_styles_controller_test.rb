require "test_helper"

# Estilo do editor (Configurações > Aparência): plain, como o VS Code, ou styled, como o source do Obsidian.
class EditorStylesControllerTest < ActionDispatch::IntegrationTest
  setup do
    use_fake_api(notes: [])
    sign_in_as
  end

  test "update keeps the style in a permanent cookie and goes back" do
    patch editor_style_path, params: { editor_style: "styled" }, headers: { "HTTP_REFERER" => "http://www.example.com/settings/appearance" }

    assert_redirected_to "http://www.example.com/settings/appearance"
    assert_equal "styled", cookies[:editor_style]
    assert_match(/expires=/i, response.headers["set-cookie"].to_s)
  end

  test "update rejects an unknown style" do
    patch editor_style_path, params: { editor_style: "fancy" }

    assert_response :bad_request
    assert_match(/"fancy".*plain or styled/, response.body)
    assert_nil cookies[:editor_style]
  end

  test "the appearance offers both styles with plain pressed by default" do
    get settings_appearance_path

    assert_select "main [role=group][aria-labelledby=editor_style_heading]" do
      assert_select "form[action=?] button[aria-pressed=true]", editor_style_path, text: "Plain"
      assert_select "form[action=?] button[aria-pressed=false]", editor_style_path, text: "Styled"
    end
  end

  test "the layout tells the editor the style from the cookie" do
    cookies[:editor_style] = "styled"

    get root_path

    assert_select "html[data-editor-style=styled]"
  end
end
