require "test_helper"

# Sino da barra de atividades (ADR-0049): o contador em frames que o frame_poll recarrega e a lista num popover, com
# aceitar e recusar pelas rotas de my_invites.
class NotificationsControllerTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  HOME_LAB = { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } }.freeze
  INVITE = { id: "invite-1", tenant_id: "tenant-2", email: EMAIL, role: "member", expires_at: "2099-10-23T12:00:00Z" }.freeze

  test "the activity bar holds the bell with a lazy badge frame polled by frame-poll" do
    use_fake_api
    sign_in_as

    get root_path

    %w[side drawer].each do |placement|
      assert_select ".activity-bar-#{placement} .activity-bar-actions button[popovertarget=notification_menu][aria-haspopup=dialog]" do
        assert_select "turbo-frame#notification_badge_#{placement}[src=?][loading=lazy][refresh=morph][data-controller=frame-poll]", notifications_path
      end
    end
  end

  test "the popover loads the list lazily and reloads it when it opens again" do
    use_fake_api
    sign_in_as

    get root_path

    assert_select "#notification_menu[popover][role=dialog][aria-labelledby=notification_menu_title]" \
                  "[data-controller=frame-poll][data-frame-poll-interval-value='0'][data-action~='toggle->frame-poll#reload']" do
      assert_select "#notification_menu_title", "Notifications"
      assert_select "turbo-frame#notification_list[src=?][loading=lazy][target=_top][data-frame-poll-target=frame]", notifications_path
    end
  end

  test "the badge shows the count and names it for screen readers in a polite live region" do
    use_fake_api(tenants: [ FakeMarkuppTenants::DEFAULT_TENANTS.sole, HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    get notifications_path, headers: { "Turbo-Frame" => "notification_badge_side" }

    assert_response :success
    assert_select "turbo-frame#notification_badge_side" do
      assert_select "[aria-live=polite]", "Notifications, 1 pending"
      assert_select ".notification-badge[aria-hidden=true]", "1"
    end
  end

  test "without notifications the badge has no count" do
    use_fake_api
    sign_in_as

    get notifications_path, headers: { "Turbo-Frame" => "notification_badge_drawer" }

    assert_select "turbo-frame#notification_badge_drawer [aria-live=polite]", "Notifications"
    assert_select ".notification-badge", count: 0
  end

  test "the list frame shows each invite with accept and decline" do
    use_fake_api(tenants: [ FakeMarkuppTenants::DEFAULT_TENANTS.sole, HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    get notifications_path, headers: { "Turbo-Frame" => "notification_list" }

    assert_select "turbo-frame#notification_list[target=_top] li", text: /Join Home lab as member/ do
      assert_select "p#notification_invite_invite-1", text: /Join Home lab/
      assert_select "form[action=?] input[name=_method][value=patch]", my_invite_path("invite-1")
      assert_select "form[action=?] button[aria-describedby=notification_invite_invite-1]", my_invite_path("invite-1"), text: "Accept"
      assert_select "form[action=?] input[name=_method][value=delete]", my_invite_path("invite-1")
      assert_select "form[action=?] button", my_invite_path("invite-1"), text: "Decline"
    end
  end

  test "the list frame says when nothing needs attention" do
    use_fake_api
    sign_in_as

    get notifications_path, headers: { "Turbo-Frame" => "notification_list" }

    assert_select "turbo-frame#notification_list", text: /Nothing needs your attention\./
  end

  test "the page without a frame lists the notifications under a heading" do
    use_fake_api(tenants: [ FakeMarkuppTenants::DEFAULT_TENANTS.sole, HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    get notifications_path

    assert_response :success
    assert_select "main h1", "Notifications"
    assert_select "main li", text: /Join Home lab as member/
  end

  test "an unreachable API leaves the badge without a count and says so in the list" do
    api = use_fake_api(invites: [ INVITE ])
    sign_in_as
    api.cut_connection!

    get notifications_path, headers: { "Turbo-Frame" => "notification_badge_side" }

    assert_response :bad_gateway
    assert_select "turbo-frame#notification_badge_side [aria-live=polite]", "Notifications"
    assert_select "turbo-frame#notification_badge_side a", count: 0
    assert_select "turbo-frame#notification_list", text: /Can't reach the markupp server/
  end

  test "a frame without a session keeps links out of the badge and leads to the login from the list" do
    get notifications_path, headers: { "Turbo-Frame" => "notification_badge_side" }

    assert_response :unauthorized
    assert_select "turbo-frame#notification_badge_side a", count: 0
    assert_select "turbo-frame#notification_list a[href=?][data-turbo-frame=_top]", new_session_path
  end

  test "accepting from the list joins the tenant like the tenant switcher" do
    use_fake_api(tenants: [ HOME_LAB ], invites: [ INVITE ])
    sign_in_as

    patch my_invite_path("invite-1")
    follow_redirect!

    assert_select "[role=status]", "Joined Home lab as member."
    get notifications_path, headers: { "Turbo-Frame" => "notification_badge_side" }
    assert_select ".notification-badge", count: 0
  end
end
