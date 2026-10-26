require "application_system_test_case"

# Sino no pé da barra de atividades (ADR-0049): o contador dos convites pendentes, que se atualiza sozinho, e a lista
# num popover, onde aceitar ou recusar responde como no seletor de tenant.
class NotificationsTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
              { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } } ].freeze
  INVITE = { id: "invite-1", tenant_id: "tenant-2", email: EMAIL, role: "member" }.freeze
  BELL = ".activity-bar-side button[popovertarget=notification_menu]".freeze

  test "the bell counts the pending invites and accepting from its popover opens the new tenant" do
    use_fake_api(tenants: TENANTS, invites: [ INVITE ])
    sign_in_through_form

    assert_selector "#{BELL} .notification-badge", text: "1"
    assert_equal "Notifications, 1 pending", find(BELL).native.accessible_name
    find(BELL).click
    within("#notification_menu") do
      assert_text "Join Home lab as member"
      click_on "Accept"
    end

    assert_text "Joined Home lab as member."
    assert_selector "nav #current_tenant", text: /Home lab.*member/m
    assert_selector "#{BELL} [aria-live=polite]", text: "Notifications", visible: :all
    assert_no_selector "#{BELL} .notification-badge"
  end

  test "declining from the popover drops the invite and the count" do
    use_fake_api(tenants: TENANTS, invites: [ INVITE ])
    sign_in_through_form

    find(BELL).click
    within("#notification_menu") { click_on "Decline" }

    assert_text "Invite declined."
    assert_no_selector "#{BELL} .notification-badge"
    find(BELL).click
    within("#notification_menu") { assert_text "Nothing needs your attention." }
  end

  test "an invite that arrives later shows up when the tab comes back" do
    api = use_fake_api(tenants: TENANTS)
    sign_in_through_form
    assert_selector "#{BELL} turbo-frame[complete]"
    assert_no_selector "#{BELL} .notification-badge"

    api.receive_invite!(INVITE)
    execute_script("document.dispatchEvent(new Event('visibilitychange'))")

    assert_selector "#{BELL} .notification-badge", text: "1"
  end

  test "the popover shows the invite that arrived after it was first opened" do
    api = use_fake_api(tenants: TENANTS)
    sign_in_through_form
    find(BELL).click
    within("#notification_menu") { assert_text "Nothing needs your attention." }
    send_keys :escape
    assert_no_selector "#notification_menu:popover-open"

    api.receive_invite!(INVITE)
    find(BELL).click

    within("#notification_menu") { assert_text "Join Home lab as member" }
  end

  test "esc closes the popover and gives the focus back to the bell" do
    use_fake_api(tenants: TENANTS, invites: [ INVITE ])
    sign_in_through_form
    find(BELL).send_keys(:enter)
    assert_selector "#notification_menu:popover-open"
    within("#notification_menu") { find_button("Accept").send_keys(:escape) }

    assert_no_selector "#notification_menu:popover-open"
    assert_selector "#{BELL}:focus"
  end
end
