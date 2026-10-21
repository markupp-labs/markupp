require "application_system_test_case"

class EmailConfirmationsTest < ApplicationSystemTestCase
  UNVERIFIED = { email: "caio@markupp.dev", password: "senha-certa", verified: false }.freeze

  test "an account without tenant confirms its email from the onboarding and sees its invite" do
    api = use_fake_api(accounts: [ UNVERIFIED ], tenants: [ FakeMarkuppTenants::DEFAULT_TENANTS.first ],
                       invites: [ { id: "invite-1", tenant_id: "tenant-1", email: UNVERIFIED[:email], role: "member" } ])
    sign_in_through_form(UNVERIFIED[:email])

    click_on "Confirm your email"
    click_on "Send confirmation email"
    assert_text "Confirmation email sent."
    click_on "I have a token"
    fill_in "Token", with: api.sent_tokens.last.fetch(:token)
    click_on "Confirm"
    assert_text "Email confirmed."

    visit tenants_path
    assert_text "Join Acme research as member"
  end
end
