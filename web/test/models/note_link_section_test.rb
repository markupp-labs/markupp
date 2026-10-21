require "test_helper"

class NoteLinkSectionTest < ActiveSupport::TestCase
  PLAN = { id: "plan", title: "plan", path: "plan.md" }.freeze

  setup do
    @api = FakeMarkuppApi.new(graph: FakeCrowdedNoteLinks.new(PLAN, count: 130).graph)
  end

  test "page brings a full page of each section from the start" do
    pages = NoteLinkSection::NAMES.map { NoteLinkSection.page(@api, "plan", it) }

    assert_equal [ 100 ] * 4, pages.map(&:size)
    assert_equal "backlink 001", pages.first.first.source.title
  end

  test "page from an offset brings the rest of the section" do
    rest = NoteLinkSection.page(@api, "plan", "suggestions", offset: 100)

    assert_equal [ "suggestion 101", "suggestion 130" ], [ rest.first.note.title, rest.last.note.title ]
  end
end
