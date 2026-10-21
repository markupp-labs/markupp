require "test_helper"

class Note::TreeTest < ActiveSupport::TestCase
  test "places notes under nested folders taken from the path" do
    tree = Note::Tree.build([ note("work/2026/plan.md"), note("work/ideas.md"), note("inbox.md") ])

    assert_equal %w[inbox.md], tree.notes.map(&:name)
    work = tree.folders.sole
    assert_equal "work", work.name
    assert_equal %w[ideas.md], work.notes.map(&:name)
    assert_equal %w[plan.md], work.folders.sole.notes.map(&:name)
  end

  test "sorts folders and notes by name" do
    tree = Note::Tree.build([ note("b/x.md"), note("a/x.md"), note("z.md"), note("c.md") ])

    assert_equal %w[a b], tree.folders.map(&:name)
    assert_equal %w[c.md z.md], tree.notes.map(&:name)
  end

  test "an empty listing is an empty root" do
    tree = Note::Tree.build([])

    assert_empty tree.folders
    assert_empty tree.notes
  end

  private
    def note(path) = Note.new(path:)
end
