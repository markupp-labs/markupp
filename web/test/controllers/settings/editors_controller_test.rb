require "test_helper"

# Configurações, seção Editor (como Settings > Editor no Obsidian): o estilo do editor e o modo Vim, para todos os papéis.
module Settings
  class EditorsControllerTest < ActionDispatch::IntegrationTest
    test "show asks for login first" do
      get "/settings/editor"

      assert_redirected_to new_session_path
    end

    test "show offers the editor style and vim keys with their hints" do
      use_fake_api(tenants: AccountsControllerTest::MEMBER_ONLY)
      sign_in_as

      get "/settings/editor"

      assert_response :success
      assert_select "main h1", "Editor"
      assert_select "main nav a[href='/settings/editor'][aria-current=page]", text: "Editor"
      assert_select "main #editor_style_heading", "Editor style"
      assert_select "main [role=group][aria-labelledby=editor_style_heading][aria-describedby=editor_style_hint] form[action=?]", editor_style_path, count: 2
      assert_select "main #editor_style_hint", text: /Styled makes headings larger/
      assert_select "main #vim_keys_heading", "Vim key bindings"
      assert_select "main form[action=?] button[role=switch][aria-labelledby=vim_keys_heading][aria-describedby=vim_keys_hint]", vim_keys_path
      assert_select "main #vim_keys_hint", text: /normal, insert and visual modes of Vim/
    end

    test "show works for an account without tenant" do
      use_fake_api(tenants: [])
      sign_in_as

      get "/settings/editor"

      assert_response :success
      assert_select "main h1", "Editor"
    end
  end
end
