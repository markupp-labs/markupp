require "application_system_test_case"

class TenantInvitesTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  ACME = { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } }.freeze

  test "without email the admin issues an invite and gets the token to pass along" do
    use_fake_api(tenants: [ ACME ], email_enabled: false)
    sign_in_through_form

    visit settings_members_path
    within("main") { click_on "Invites" }
    fill_in "Email", with: "gil@markupp.dev"
    select "viewer", from: "Role"
    click_on "Send invite"

    assert_text "Send this token to gil@markupp.dev yourself"
    assert_field "Invite token", with: "issued-token-1", readonly: true
  end

  test "the admin revokes a pending invite after confirming" do
    use_fake_api(tenants: [ ACME ], invites: [ { id: "invite-1", tenant_id: "tenant-1", email: "dani@markupp.dev", role: "member" } ])
    sign_in_through_form

    visit settings_invites_path
    accept_panel_confirm("Revoke the invite for dani@markupp.dev?", button: "Revoke") { click_on "Revoke" }

    assert_text "Invite revoked."
    assert_no_text "dani@markupp.dev"
  end
end
