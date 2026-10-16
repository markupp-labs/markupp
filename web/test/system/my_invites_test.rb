require "application_system_test_case"

class MyInvitesTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "accepting an invite from the tenant switcher opens the new tenant" do
    use_fake_api(tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                            { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } } ],
                 invites: [ { id: "invite-1", tenant_id: "tenant-2", email: EMAIL, role: "member" } ])
    sign_in_through_form

    click_on "Acme research"
    within("#tenant_switcher") do
      assert_text "Join Home lab as member"
      click_on "Accept"
    end

    assert_text "Joined Home lab as member."
    assert_selector "nav button[popovertarget=tenant_switcher]", text: /Home lab.*member/m
  end

  test "declining an invite on the onboarding drops it" do
    use_fake_api(tenants: [ { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } } ],
                 invites: [ { id: "invite-1", tenant_id: "tenant-2", email: EMAIL, role: "viewer" } ])
    sign_in_through_form

    assert_text "Join Home lab as viewer"
    click_on "Decline"

    assert_text "Invite declined."
    assert_no_text "Join Home lab"
  end
end
