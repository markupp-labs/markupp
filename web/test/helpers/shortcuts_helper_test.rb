require "test_helper"

class ShortcutsHelperTest < ActionView::TestCase
  test "shortcut_label spaces the keys out and shortens Delete" do
    assert_equal "Alt Shift C", shortcut_label("Alt+Shift+C")
    assert_equal "Del", shortcut_label("Delete")
    assert_equal "Ctrl ,", shortcut_label("Ctrl+,")
  end
end
