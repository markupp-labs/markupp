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

  test "current_sidebar_view reads the view from its cookie" do
    cookies[:sidebar_view] = "graph"

    assert_equal "graph", current_sidebar_view
  end

  test "current_sidebar_view falls back to the explorer without a known view" do
    assert_equal "explorer", current_sidebar_view

    cookies[:sidebar_view] = "outline"
    assert_equal "explorer", current_sidebar_view
  end
end
