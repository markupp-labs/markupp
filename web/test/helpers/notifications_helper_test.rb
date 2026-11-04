require "test_helper"

class NotificationsHelperTest < ActionView::TestCase
  test "the badge shows the count up to 99" do
    assert_equal "1", notification_badge_count(1)
    assert_equal "99", notification_badge_count(99)
  end

  test "past 99 the badge stops growing, so it fits on the 48px bell" do
    assert_equal "99+", notification_badge_count(100)
  end

  test "the label names the count, or only the bell when nothing is pending" do
    assert_equal "Notifications", notification_label(0)
    assert_equal "Notifications, 1 pending", notification_label(1)
    assert_equal "Notifications, 120 pending", notification_label(120)
  end
end
