require "test_helper"

class SidebarHelperTest < ActionView::TestCase
  test "sidebar_width keeps a width inside the limits" do
    assert_equal 320, sidebar_width("320")
  end

  test "sidebar_width clamps a width outside the limits" do
    assert_equal 480, sidebar_width("9999")
    assert_equal 240, sidebar_width("10")
  end

  test "sidebar_width falls back to the default without a numeric width" do
    assert_nil sidebar_width(nil)
    assert_nil sidebar_width("wide")
  end
end
