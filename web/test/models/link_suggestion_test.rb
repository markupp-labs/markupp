require "test_helper"

class LinkSuggestionTest < ActiveSupport::TestCase
  test "for_note builds the notes the note could link, with the scores" do
    api = FakeMarkuppApi.new(graph: { suggestions: { "1" => [
      { note: { id: "3", title: "cluster", path: "cluster.md" }, score: 0.0325, resource_allocation: 0.5, cosine: 0.38 }
    ] } })

    suggestions = LinkSuggestion.for_note(api, "1")
    assert_equal 1, suggestions.size
    suggestion = suggestions.first

    assert_equal [ "3", "cluster.md" ], [ suggestion.note.id, suggestion.note.path ]
    assert_equal [ 0.0325, 0.5, 0.38 ], [ suggestion.score, suggestion.resource_allocation, suggestion.cosine ]
  end
end
