require "test_helper"

class LinkTest < ActiveSupport::TestCase
  PLAN = { id: "1", title: "plan", path: "work/plan.md" }.freeze
  MEETING = { id: "2", title: "meeting", path: "meeting.md" }.freeze

  setup do
    @api = FakeMarkuppApi.new(graph: {
      links: { "1" => [
        { source: PLAN, target: MEETING, cited_title: "Meeting", type: "explicit", predicate: nil, inferred_target: nil },
        { source: PLAN, target: MEETING, cited_title: "meeting", type: "typed", predicate: "author", inferred_target: nil },
        { source: PLAN, target: nil, cited_title: "Meetng", type: "explicit", predicate: nil, inferred_target: MEETING }
      ] },
      backlinks: { "2" => [ { source: PLAN, target: MEETING, cited_title: "Meeting", type: "explicit", predicate: nil, inferred_target: nil } ] }
    })
  end

  test "from_note builds the links the note makes, with the notes on both ends" do
    links = Link.from_note(@api, "1")
    assert_equal 3, links.size
    link = links.first

    assert_equal [ "1", "plan", "work/plan.md" ], [ link.source.id, link.source.title, link.source.path ]
    assert_equal [ "2", "meeting.md" ], [ link.target.id, link.target.path ]
    assert_equal [ "Meeting", "explicit", nil ], [ link.cited_title, link.type, link.predicate ]
  end

  test "an unresolved link has no target and may carry the inferred one" do
    links = Link.from_note(@api, "1")
    assert_equal 3, links.size
    link = links.last

    assert_nil link.target
    assert link.unresolved?
    assert_equal "2", link.inferred_target.id
  end

  test "a typed link keeps its frontmatter key as predicate" do
    links = Link.from_note(@api, "1")
    assert_equal 3, links.size
    link = links.second

    assert_equal [ "typed", "author" ], [ link.type, link.predicate ]
    assert_not link.unresolved?
  end

  test "from_note filters by type and pages" do
    assert_equal [ "author" ], Link.from_note(@api, "1", type: "typed").map(&:predicate)
    assert_equal [ "Meetng" ], Link.from_note(@api, "1", offset: 2, limit: 1).map(&:cited_title)
  end

  test "to_note builds the backlinks from other notes" do
    assert_equal [ "work/plan.md" ], Link.to_note(@api, "2").map { it.source.path }
  end

  test "only the links by mention, by similar text and by meaning are automatic" do
    mentioned = Link.from_api({ "source" => PLAN.stringify_keys, "target" => MEETING.stringify_keys, "cited_title" => "meeting", "type" => "mentioned",
                                "predicate" => nil, "inferred_target" => nil })
    related = Link.from_api({ "source" => PLAN.stringify_keys, "target" => MEETING.stringify_keys, "cited_title" => "", "type" => "related",
                              "predicate" => nil, "inferred_target" => nil })
    semantic = Link.from_api({ "source" => PLAN.stringify_keys, "target" => MEETING.stringify_keys, "cited_title" => "", "type" => "semantic",
                               "predicate" => nil, "inferred_target" => nil })

    assert mentioned.automatic?
    assert related.automatic?
    assert semantic.automatic?
    assert_not Link.from_note(@api, "1").any?(&:automatic?)
  end
end
