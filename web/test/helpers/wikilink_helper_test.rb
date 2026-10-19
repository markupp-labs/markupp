require "test_helper"

class WikilinkHelperTest < ActionView::TestCase
  test "wikilink_targets maps each note path without .md to its url" do
    tree = Note::Tree.build([ note("1", "inbox.md"), note("2", "work/2026/plan.md"), note("3", "data.txt") ])

    assert_equal({ "inbox" => "/notes/1", "work/2026/plan" => "/notes/2", "data.txt" => "/notes/3" }, wikilink_targets(tree))
  end

  test "wikilink_targets is empty for an empty tree" do
    assert_empty wikilink_targets(Note::Tree.build([]))
  end

  private
    def note(id, path) = Note.new(id:, path:)
end
