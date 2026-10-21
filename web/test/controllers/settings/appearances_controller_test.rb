require "test_helper"

# Configurações, seção Aparência: o tema, com o escolhido marcado; sem cookie o painel segue o sistema.
module Settings
  class AppearancesControllerTest < ActionDispatch::IntegrationTest
    test "show asks for login first" do
      get settings_appearance_path

      assert_redirected_to new_session_path
    end

    test "show offers both themes and says it follows the system without a choice" do
      use_fake_api
      sign_in_as

      get settings_appearance_path

      assert_response :success
      assert_select "main h1", "Appearance"
      assert_select "main nav a[href=?][aria-current=page]", settings_appearance_path, text: "Appearance"
      assert_select "main form[action=?] button[aria-pressed=false]", theme_path, count: 2
      assert_select "main button svg[data-icon=sun]"
      assert_select "main button svg[data-icon=moon]"
      assert_select "main", text: /Follows your system theme until you pick one/
      assert_select ".activity-bar-side a[href=?][aria-label=Settings][aria-current=page]", settings_path
    end

    test "show marks the theme in the cookie as pressed" do
      use_fake_api
      sign_in_as
      cookies[:theme] = "dark"

      get settings_appearance_path

      assert_select "main button[aria-pressed=true]", text: "Dark"
      assert_select "main button[aria-pressed=false]", text: "Light"
      assert_select "main", text: /Follows your system/, count: 0
    end

    test "show holds only the theme, without the editor style or vim keys" do
      use_fake_api
      sign_in_as

      get settings_appearance_path

      assert_select "main section form[action=?]", editor_style_path, count: 0
      assert_select "main section form[action=?]", vim_keys_path, count: 0
      assert_select "main section", text: /Vim key/, count: 0
    end

    test "show offers every locale by its own name and says it follows the browser without a choice" do
      use_fake_api
      sign_in_as

      get settings_appearance_path

      assert_select "main h2", "Language"
      assert_select "main form[action=?][data-turbo=false] button[aria-pressed=false]", locale_path, count: 2
      assert_select "main button", text: "English"
      assert_select "main button", text: "Português (Brasil)"
      assert_select "main", text: /Follows your browser language until you pick one/
    end

    test "show marks the locale in the cookie as pressed, in that locale" do
      use_fake_api
      sign_in_as
      cookies[:locale] = "pt-BR"

      get settings_appearance_path

      assert_select "main h2", "Idioma"
      assert_select "main button[aria-pressed=true]", text: "Português (Brasil)"
      assert_select "main button[aria-pressed=false]", text: "English"
      assert_select "main", text: /Segue o idioma do navegador/, count: 0
    end

    test "picking a theme comes back to the appearance" do
      use_fake_api
      sign_in_as

      patch theme_path, params: { theme: "dark" }, headers: { "HTTP_REFERER" => settings_appearance_url }

      assert_redirected_to settings_appearance_url
    end
  end
end
