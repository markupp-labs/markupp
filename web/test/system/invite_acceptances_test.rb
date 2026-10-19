require "application_system_test_case"

class InviteAcceptancesTest < ApplicationSystemTestCase
  test "accepting an invite by token from the login creates the account and opens the tenant" do
    use_fake_api(invites: [ { id: "invite-1", tenant_id: "tenant-1", email: "caio@markupp.dev", role: "member", token: "tok-1" } ])

    visit new_session_path
    click_on "Accept it here."
    fill_in "Token", with: "tok-1"
    fill_in "Password", with: "senha-nova"
    click_on "Create account"

    assert_selector "nav", text: "caio@markupp.dev"
    assert_selector "nav #current_tenant", text: /Acme research.*member/m
  end
end
