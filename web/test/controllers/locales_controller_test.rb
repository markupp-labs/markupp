require "test_helper"

# Idioma escolhido (Configurações > Aparência ou paleta): fica num cookie e vale mais que o Accept-Language.
class LocalesControllerTest < ActionDispatch::IntegrationTest
  setup do
    use_fake_api(notes: [])
    sign_in_as
  end

  test "update keeps the locale in a permanent cookie and goes back" do
    patch locale_path, params: { locale: "pt-BR" }, headers: { "HTTP_REFERER" => "http://www.example.com/settings/appearance" }

    assert_redirected_to "http://www.example.com/settings/appearance"
    assert_equal "pt-BR", cookies[:locale]
    assert_match(/expires=/i, response.headers["set-cookie"].to_s)
  end

  test "update without a referer goes to the appearance settings" do
    patch locale_path, params: { locale: "en" }

    assert_redirected_to "/settings/appearance"
  end

  test "update rejects a locale the panel does not speak" do
    patch locale_path, params: { locale: "fr" }

    assert_response :bad_request
    assert_match(/"fr".*en or pt-BR/, response.body)
    assert_nil cookies[:locale]
  end
end
