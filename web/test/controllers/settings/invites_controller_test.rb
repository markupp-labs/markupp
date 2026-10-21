require "test_helper"

# Configurações do tenant, aba Convites: emitir, listar e revogar, com o token na tela quando o servidor não manda email.
module Settings
  class InvitesControllerTest < ActionDispatch::IntegrationTest
    ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
    BIA = "bia@markupp.dev"
    PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
    ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: BIA, password: PASSWORD } ].freeze
    ACME = { id: "tenant-1", name: "Acme research", members: { ANA => "owner", BIA => "admin", "caio@markupp.dev" => "member" } }.freeze
    INVITES = [
      { id: "invite-1", tenant_id: "tenant-1", email: "eva@markupp.dev", role: "member", expires_at: "2099-10-23T12:00:00Z", accepted_at: "2026-10-02T12:00:00Z" },
      { id: "invite-2", tenant_id: "tenant-1", email: "fabi@markupp.dev", role: "viewer", expires_at: "2020-01-08T12:00:00Z" },
      { id: "invite-3", tenant_id: "tenant-1", email: "dani@markupp.dev", role: "admin", expires_at: "2099-10-23T12:00:00Z" }
    ].freeze

    test "the invites section is current in the settings nav" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get settings_invites_path

      assert_response :success
      assert_select "main h1", "Invites"
      assert_select "main nav a[href=?][aria-current=page]", settings_invites_path, text: "Invites"
    end

    test "lists the pending invites first with expiry and revoke, then the answered ones with their status" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], invites: INVITES)
      sign_in_as

      get settings_invites_path

      emails = css_select("main li[data-invite-status]").map { it.text[/\S+@markupp\.dev/] }
      assert_equal %w[dani@markupp.dev eva@markupp.dev fabi@markupp.dev], emails
      assert_select "main li[data-invite-status=pending]", text: /dani@markupp\.dev.*admin.*Expires Oct 23/m do
        assert_select "form[action=?] button.btn-danger[data-turbo-confirm=?]", settings_invite_path("invite-3"), "Revoke the invite for dani@markupp.dev?", text: "Revoke"
      end
      assert_select "main li[data-invite-status=accepted].text-ink-muted", text: /Accepted/ do
        assert_select "form", count: 0
      end
      assert_select "main li[data-invite-status=expired].text-ink-muted", text: /Expired/
    end

    test "says how to start when there are no invites" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get settings_invites_path

      assert_select "main", text: /No invites yet\. Invite someone with the form above\./
    end

    test "the form offers the roles up to your own with member chosen" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as(BIA)

      get settings_invites_path

      assert_select "form[action=?]", settings_invites_path do
        assert_select "label", "Email"
        assert_select "input[type=email][name=?]", "invite[email]"
        assert_select "label", "Role"
        assert_equal %w[admin member viewer], css_select("select[name='invite[role]'] option").map { it["value"] }
        assert_select "select option[selected]", "member"
        assert_select "button", "Send invite"
      end
    end

    test "sending an invite through email says so and lists it" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      post settings_invites_path, params: { invite: { email: "gil@markupp.dev", role: "member" } }

      assert_redirected_to settings_invites_path
      follow_redirect!
      assert_select "[role=status]", "Invite sent to gil@markupp.dev."
      assert_select "main li[data-invite-status=pending]", text: /gil@markupp\.dev/
    end

    test "without email the token shows once for the admin to pass along" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], email_enabled: false)
      sign_in_as

      post settings_invites_path, params: { invite: { email: "gil@markupp.dev", role: "member" } }

      assert_response :created
      expires = I18n.l(7.days.from_now.to_date, format: :short)
      assert_select "main [role=status]",
                    text: /This server has no email set up\. Send this token to gil@markupp\.dev yourself; it works once and expires on #{expires}\./
      assert_select "main label", "Invite token"
      assert_select "main input[readonly][value=issued-token-1]"
      assert_select "main li[data-invite-status=pending]", text: /gil@markupp\.dev/

      get settings_invites_path
      assert_select "input[value=issued-token-1]", count: 0
    end

    test "the page with the token stays out of the Turbo cache" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], email_enabled: false)
      sign_in_as

      post settings_invites_path, params: { invite: { email: "gil@markupp.dev", role: "member" } }

      assert_select "head meta[name=turbo-cache-control][content=no-cache]"
    end

    test "an address the API refuses shows under the email field" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      post settings_invites_path, params: { invite: { email: "gil", role: "member" } }

      assert_response :unprocessable_content
      assert_select "input[name='invite[email]'][aria-invalid=true][value=gil][aria-describedby=invite_email_error]"
      assert_select "#invite_email_error", "Enter an address like name@example.com."
    end

    test "an email already in the tenant shows under the email field" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      post settings_invites_path, params: { invite: { email: "caio@markupp.dev", role: "member" } }

      assert_response :conflict
      assert_select "#invite_email_error", "caio@markupp.dev is already a member."
    end

    test "a role above your own shows under the role field" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as(BIA)

      post settings_invites_path, params: { invite: { email: "gil@markupp.dev", role: "owner" } }

      assert_response :forbidden
      assert_select "select[name='invite[role]'][aria-invalid=true][aria-describedby=invite_role_error]"
      assert_select "#invite_role_error", "You can't give or change a role above your own."
    end

    test "revoking a pending invite drops it" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], invites: INVITES)
      sign_in_as

      delete settings_invite_path("invite-3")

      assert_redirected_to settings_invites_path
      follow_redirect!
      assert_select "[role=status]", "Invite revoked."
      assert_select "main li", text: /dani@markupp\.dev/, count: 0
    end

    test "revoking an invite already answered says so" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], invites: INVITES)
      sign_in_as

      delete settings_invite_path("invite-1")

      assert_redirected_to settings_invites_path
      follow_redirect!
      assert_select "[role=alert]", "That invite is no longer pending."
    end

    test "a member is sent back to the notes" do
      use_fake_api(tenants: [ ACME.merge(members: { ANA => "member" }) ])
      sign_in_as

      post settings_invites_path, params: { invite: { email: "gil@markupp.dev", role: "member" } }

      assert_redirected_to root_path
    end

    test "answers bad gateway when the API is unreachable" do
      api = use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as
      api.cut_connection!

      get settings_invites_path

      assert_response :bad_gateway
      assert_select "main", text: /Can't reach the markupp server/
    end
  end
end
