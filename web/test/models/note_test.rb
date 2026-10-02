require "test_helper"

class NoteTest < ActiveSupport::TestCase
  test "all builds notes from the API listing" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan", updated_at: "2026-09-30T12:00:00Z" } ])

    note = Note.all.sole
    assert_equal "1", note.id
    assert_equal "work/plan.md", note.path
    assert_equal Time.utc(2026, 9, 30, 12), note.updated_at
  end

  test "name is the last segment of the path" do
    assert_equal "plan.md", Note.new(path: "work/2026/plan.md").name
  end

  test "folder_names are the segments before the name, ignoring empty ones" do
    assert_equal %w[work 2026], Note.new(path: "/work//2026/plan.md").folder_names
    assert_empty Note.new(path: "plan.md").folder_names
  end
end
