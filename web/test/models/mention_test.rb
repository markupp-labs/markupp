require "test_helper"

class MentionTest < ActiveSupport::TestCase
  setup do
    @api = FakeMarkuppApi.new(graph: {
      mentions: { "1" => [ { note: { id: "2", title: "kubernetes", path: "infra/kubernetes.md" }, cited_title: "kubernetes", prior: 0.33 } ] },
      backmentions: { "2" => [ { note: { id: "1", title: "plan", path: "plan.md" }, cited_title: "kubernetes", prior: 0.33 } ] }
    })
  end

  test "made_by builds the notes whose title the note cites without linking" do
    mentions = Mention.made_by(@api, "1")
    assert_equal 1, mentions.size
    mention = mentions.first

    assert_equal [ "2", "kubernetes", "infra/kubernetes.md" ], [ mention.note.id, mention.note.title, mention.note.path ]
    assert_equal [ "kubernetes", 0.33 ], [ mention.cited_title, mention.prior ]
  end

  test "citing builds the notes that cite a title of the note without linking" do
    assert_equal [ "plan.md" ], Mention.citing(@api, "2").map { it.note.path }
  end
end
