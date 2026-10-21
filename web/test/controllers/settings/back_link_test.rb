require "test_helper"

# Configurações: além do fechar (raiz), um voltar para a página de onde se veio, que sobrevive à troca de aba.
module Settings
  class BackLinkTest < ActionDispatch::IntegrationTest
    setup do
      use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
      sign_in_as
    end

    test "back leads to the page the settings were opened from" do
      get settings_account_path, headers: { "HTTP_REFERER" => "http://www.example.com/notes/1?mode=read" }

      assert_select "main a[href=?][aria-label=?][aria-keyshortcuts=Escape]", "/notes/1?mode=read", "Back"
      assert_select "main a[href=?][aria-label=?]", root_path, "Close settings"
    end

    test "back keeps the origin while moving between settings sections" do
      get settings_account_path, headers: { "HTTP_REFERER" => "http://www.example.com/notes/1" }
      get settings_appearance_path, headers: { "HTTP_REFERER" => "http://www.example.com/settings/account" }

      assert_select "main a[href=?][aria-label=?]", "/notes/1", "Back"
    end

    test "back ignores a referer from another site and falls back to the root" do
      get settings_account_path, headers: { "HTTP_REFERER" => "https://evil.example.net/phish" }

      assert_select "main a[href=?][aria-label=?]", root_path, "Back"
    end

    test "back falls back to the root without a referer" do
      get settings_account_path

      assert_select "main a[href=?][aria-label=?]", root_path, "Back"
    end
  end
end
