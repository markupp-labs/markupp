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

    test "the sidebar shows the note tree and new note, as on the note pages" do
      use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "" } ])
      sign_in_as

      get settings_account_path

      assert_select "nav#sidebar .sidebar-scroll" do
        assert_select "a[href=?]", new_note_path, text: "New note"
        assert_select "details summary", text: "work"
        assert_select "a[href=?]", note_path("1"), text: "plan.md"
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

    test "without tenant the sidebar has no note tree" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_account_path

      assert_response :success
      assert_select "nav#sidebar a[href=?]", new_note_path, count: 0
    end

    test "a close button leads back to the notes" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "main a[href=?][aria-label=?][title=?] svg[data-icon=x]", root_path, "Close settings", "Close settings"
      assert_select "nav#sidebar a", text: "Back to notes", count: 0
    end

    test "a member sees account and appearance, without the tenant section" do
      use_fake_api(tenants: MEMBER_ONLY)
      sign_in_as

      get settings_account_path

      assert_select "main nav[aria-label=Settings]" do
        assert_select "a[href=?][aria-current=page]", settings_account_path, text: "Account"
        assert_select "a[href=?]:not([aria-current])", settings_appearance_path, text: "Appearance"
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
      end
    end

    test "the settings links live only in the settings nav, not in the sidebar" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "nav#sidebar .sidebar-scroll a[href^='/settings']", count: 0
    end

    test "the sidebar footer marks the email link as current" do
      use_fake_api
      sign_in_as

      get settings_account_path

      assert_select "nav footer a[href=?][aria-current=page]", settings_account_path, text: EMAIL
    end

    test "show works for an account without tenant" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_account_path

      assert_response :success
      assert_select "main nav a[href=?]", settings_appearance_path
      assert_select "nav footer a[href=?]", settings_account_path
    end

    test "the empty invite list of the onboarding points to the account" do
      use_fake_api(tenants: [])
      sign_in_as

      get tenants_path

      assert_select "main a[href=?]", settings_account_path, text: "Confirm your email"
    end
  end
end
