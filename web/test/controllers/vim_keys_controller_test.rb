require "test_helper"

# Modo Vim do editor (Configuracoes > Aparencia > Editor): desligado por padrao, ligado ou desligado num cookie.
class VimKeysControllerTest < ActionDispatch::IntegrationTest
  setup do
    use_fake_api(notes: [])
    sign_in_as
  end

  test "update keeps the choice in a permanent cookie and goes back" do
    patch "/vim_keys", params: { vim_keys: "on" }, headers: { "HTTP_REFERER" => "http://www.example.com/settings/appearance" }

    assert_redirected_to "http://www.example.com/settings/appearance"
    assert_equal "on", cookies[:vim_keys]
    assert_match(/expires=/i, response.headers["set-cookie"].to_s)
  end

  test "update rejects an unknown value" do
    patch "/vim_keys", params: { vim_keys: "emacs" }

    assert_response :bad_request
    assert_match(/"emacs".*on or off/, response.body)
    assert_nil cookies[:vim_keys]
  end

  test "the appearance offers vim keys next to the editor style, off by default and outside Turbo" do
    get settings_appearance_path

    assert_select "main form[action='/vim_keys'][data-turbo=false]" do
      assert_select "input[name=vim_keys][value=on]", 1
      assert_select "button[aria-pressed=false]", text: "Vim keys"
    end
  end

  test "the appearance offers to turn vim keys off when they are on" do
    cookies[:vim_keys] = "on"

    get settings_appearance_path

    assert_select "main form[action='/vim_keys']" do
      assert_select "input[name=vim_keys][value=off]", 1
      assert_select "button[aria-pressed=true]", text: "Vim keys"
    end
  end

  test "the layout tells the editor whether vim keys are on, off without a known cookie" do
    get root_path
    assert_select "html[data-vim-keys=off]"

    cookies[:vim_keys] = "on"
    get root_path
    assert_select "html[data-vim-keys=on]"
  end
end
