require "application_system_test_case"

class TenantsTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "choosing a tenant and switching it from the sidebar" do
    use_fake_api(tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                            { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ])
    sign_in_through_form

    assert_text "Choose a tenant"
    click_on "Home lab"
    assert_selector "nav button[popovertarget=tenant_switcher]", text: /Home lab.*member/m

    click_on "Home lab"
    within("#tenant_switcher") { click_on "Acme research" }

    assert_selector "nav button[popovertarget=tenant_switcher]", text: /Acme research.*owner/m
  end
end
