require "test_helper"

# O idioma vem do cookie da escolha nas configurações, depois do Accept-Language do navegador; sem nenhum que o painel
# fale, fica o inglês.
class LocaleTest < ActionDispatch::IntegrationTest
  test "without Accept-Language the panel speaks English" do
    get new_session_path

    assert_select "html[lang=en]"
    assert_select "h1", "Log in"
  end

  test "a Brazilian Portuguese browser gets the panel in pt-BR" do
    get new_session_path, headers: { "Accept-Language" => "pt-BR,pt;q=0.9,en;q=0.8" }

    assert_select "html[lang=pt-BR]"
    assert_select "h1", "Entrar"
    assert_select "label", "Senha"
  end

  test "another Portuguese region falls back to pt-BR" do
    get new_session_path, headers: { "Accept-Language" => "pt-PT" }

    assert_select "html[lang=pt-BR]"
  end

  test "a language the panel does not speak falls back to English" do
    get new_session_path, headers: { "Accept-Language" => "fr-FR,de;q=0.8" }

    assert_select "html[lang=en]"
  end

  test "the locale of one request does not leak into the next" do
    get new_session_path, headers: { "Accept-Language" => "pt-BR" }

    assert_equal :en, I18n.locale
  end

  test "the chosen locale beats the browser" do
    cookies[:locale] = "pt-BR"
    get new_session_path, headers: { "Accept-Language" => "en-US" }

    assert_select "html[lang=pt-BR]"
  end

  test "choosing English beats a Portuguese browser" do
    cookies[:locale] = "en"
    get new_session_path, headers: { "Accept-Language" => "pt-BR" }

    assert_select "html[lang=en]"
  end

  test "a cookie with a locale the panel does not speak is ignored" do
    cookies[:locale] = "fr"
    get new_session_path, headers: { "Accept-Language" => "pt-BR" }

    assert_select "html[lang=pt-BR]"
  end

  test "flash messages follow the request locale" do
    sign_in_as
    delete session_path, headers: { "Accept-Language" => "pt-BR" }
    follow_redirect! headers: { "Accept-Language" => "pt-BR" }

    assert_select "p[role=status]", /Você saiu/
  end
end
