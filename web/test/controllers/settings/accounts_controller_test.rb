require "test_helper"

# Configurações, seção Conta: email e confirmação dele, também para quem ainda não tem tenant.
module Settings
  class AccountsControllerTest < ActionDispatch::IntegrationTest
    EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
    MEMBER_ONLY = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "member" } } ].freeze

    test "show asks for login first" do
      get settings_account_path

      assert_redirected_to new_session_path
    end

    test "settings open on the account" do
      use_fake_api
      sign_in_as

      get "/settings"

      assert_redirected_to settings_account_path
    end

    test "the old account page moves into the settings" do
      get "/account"

      assert_redirected_to settings_account_path
    end

    test "show has the email and the ways to confirm it" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_response :success
      assert_select "main h1", "Account"
      assert_select "main", text: /#{EMAIL}/
      assert_select "main h2", "Email confirmation"
      assert_select "main form[action=?][method=post] button", email_verification_path, text: "Send confirmation email"
      assert_select "main a[href=?]", new_email_confirmation_path, text: "I have a token"
    end

    test "show offers the password change through the reset email" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "main h2", "Password"
      assert_select "main", text: /emails a token to #{EMAIL}/
      assert_select "main form[action=?][method=post] button", password_change_path, text: "Change password"
      assert_select "main a[href=?]", new_password_reset_path, text: "I have a reset token"
    end

    test "show hides change password when /auth/me says the account can't use a password" do
      use_fake_api(accounts: [ { email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, password_login: false } ])
      sign_in_as

      get settings_account_path

      assert_select "main form[action=?]", password_change_path, count: 0
    end

    test "show keeps change password when /auth/me says the account can use a password" do
      use_fake_api(accounts: [ { email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, password_login: true } ])
      sign_in_as

      get settings_account_path

      assert_select "main form[action=?] button", password_change_path, text: "Change password"
    end

    test "the sidebar shows the note tree and alt+n, as on the note pages" do
      use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])
      sign_in_as

      get settings_account_path

      assert_select "nav#sidebar .sidebar-scroll" do
        assert_select "[data-note-tree-new-note-url-value=?]", new_note_path
        assert_select "[role=treeitem][aria-expanded]", text: "└── work/"
        assert_select "a[href=?]", note_path("1"), text: "└── plan"
        assert_select "button[data-action='search#open']", text: /Search/
      end
    end

    test "the settings still work when the note listing fails, with a short warning in the sidebar" do
      api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
      sign_in_as
      api.cut_notes!

      get settings_account_path

      assert_response :success
      assert_select "main h1", "Account"
      assert_select "nav#sidebar .sidebar-scroll p", text: "Can't reach the markupp server"
      assert_select "nav#sidebar .sidebar-scroll a[href=?]", note_path("1"), count: 0
    end

    # Com a listagem fora a barra mostra só o aviso, sem oferecer uma nota nova.
    test "the sidebar offers no new note when the note listing fails" do
      api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
      sign_in_as
      api.cut_notes!

      get settings_account_path

      assert_select "nav#sidebar [data-note-tree-new-note-url-value]", count: 0
    end

    test "without tenant the sidebar has no note tree" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_account_path

      assert_response :success
      assert_select "nav#sidebar [data-note-tree-new-note-url-value]", count: 0
    end

    test "a close button leads back to the notes" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "main a[href=?][aria-label=?][title=?] svg[data-icon=x]", root_path, "Close settings", "Close settings"
      assert_select "nav#sidebar a", text: "Back to notes", count: 0
    end

    test "a member sees account, appearance and editor, in that order, without the tenant section" do
      use_fake_api(tenants: MEMBER_ONLY)
      sign_in_as

      get settings_account_path

      assert_select "main nav[aria-label=Settings]" do
        assert_select "a[href=?][aria-current=page]", settings_account_path, text: "Account"
        assert_select "a[href=?]:not([aria-current])", settings_appearance_path, text: "Appearance"
        assert_select "a[href='/settings/editor']:not([aria-current])", text: "Editor"
        assert_select "a" do |links|
          assert_equal %w[Account Appearance Editor], links.map { it.text.strip }
        end
        assert_select "a[href=?]", settings_members_path, count: 0
        assert_select "a[href=?]", settings_invites_path, count: 0
      end
    end

    test "an owner also sees the tenant section with members and invites" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "main nav[aria-label=Settings]" do
        assert_select "*", text: /Acme research/
        assert_select "a[href=?]", settings_members_path, text: "Members"
        assert_select "a[href=?]", settings_invites_path, text: "Invites"
        assert_select "a" do |links|
          assert_equal %w[Account Appearance Editor Members Invites], links.map { it.text.strip }
        end
      end
    end

    test "the settings links live only in the settings nav, not in the sidebar" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "nav#sidebar .sidebar-scroll a[href^='/settings']", count: 0
    end

    test "the account menu of the sidebar footer marks the account as current" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "nav footer a[href=?][aria-label=Settings][aria-current=page]", settings_path
    end

    test "show works for an account without tenant" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_account_path

      assert_response :success
      assert_select "main nav a[href=?]", settings_appearance_path
      assert_select "nav footer a[href=?][aria-label=Settings]", settings_path
    end

    test "the empty invite list of the onboarding points to the account" do
      use_fake_api(tenants: [])
      sign_in_as

      get tenants_path

      assert_select "main a[href=?]", settings_account_path, text: "Confirm your email"
    end
  end
end
