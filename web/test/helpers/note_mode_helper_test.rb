require "test_helper"

class NoteModeHelperTest < ActionView::TestCase
  test "note_mode keeps the last mode chosen for a saved note" do
    assert_equal "read", note_mode("read", persisted: true)
    assert_equal "split", note_mode("split", persisted: true)
  end

  test "note_mode opens a new note in edit whatever was chosen last" do
    assert_equal "edit", note_mode("read", persisted: false)
  end

  test "note_mode falls back to edit without a known mode" do
    assert_equal "edit", note_mode(nil, persisted: true)
    assert_equal "edit", note_mode("fullscreen", persisted: true)
  end
end
