require "test_helper"

class NoteTest < ActiveSupport::TestCase
  test "all builds notes from the API listing" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan", updated_at: "2026-09-30T12:00:00Z" } ])

    note = Note.all.sole
    assert_equal "1", note.id
    assert_equal "work/plan.md", note.path
    assert_equal Time.utc(2026, 9, 30, 12), note.updated_at
  end

  test "find builds the note with the given id" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "# B\n" } ])

    note = Note.find("2")
    assert_equal "b.md", note.path
    assert_equal "# B\n", note.content
  end

  test "find raises NotFound for an unknown id" do
    use_fake_api(notes: [])

    assert_raises(MarkuppApi::NotFound) { Note.find("missing") }
  end

  test "search builds notes from the matches without content" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "Ship it", updated_at: "2026-09-30T12:00:00Z" },
                          { id: "2", path: "inbox.md", content: "nothing" } ])

    note = Note.search("SHIP").sole
    assert_equal [ "1", "work/plan.md", Time.utc(2026, 9, 30, 12) ], [ note.id, note.path, note.updated_at ]
    assert_nil note.content
  end

  test "search encodes the query" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "a & b?" } ])

    assert_equal [ "1" ], Note.search("a & b?").map(&:id)
  end

  test "search asks for at most 20 matches" do
    use_fake_api(notes: Array.new(21) { |index| { id: index.to_s, path: "#{index}.md", content: "match" } })

    assert_equal 20, Note.search("match").size
  end

  test "search with a blank query is empty without calling the API" do
    use_fake_api(unreachable: true)

    assert_empty Note.search("")
    assert_empty Note.search(nil)
  end

  test "persisted only with an id, so routes use the id" do
    assert Note.new(id: "7").persisted?
    assert_equal "7", Note.new(id: "7").to_param
    assert_not Note.new(path: "draft.md").persisted?
  end

  test "name is the last segment of the path" do
    assert_equal "plan.md", Note.new(path: "work/2026/plan.md").name
  end

  test "folder_names are the segments before the name, ignoring empty ones" do
    assert_equal %w[work 2026], Note.new(path: "/work//2026/plan.md").folder_names
    assert_empty Note.new(path: "plan.md").folder_names
  end

  test "update sends path, content and the version read, to the millisecond" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "old" } ])
    note = Note.new(id: "1", path: "a.md", updated_at: "2026-09-30T12:00:00.123456Z")

    note.update({ content: "new" })

    assert_equal [ { path: "a.md", content: "new", lastModifiedAt: "2026-09-30T12:00:00.123Z", force: false } ], api.put_requests
    assert_equal "new", Note.find("1").content
  end

  test "update with force overwrites a newer version" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "old" } ], conflict: true)

    Note.new(id: "1", path: "a.md", updated_at: Time.utc(2026)).update({ content: "mine" }, force: true)

    assert api.put_requests.sole[:force]
    assert_equal "mine", Note.find("1").content
  end

  test "untitled is an empty note named Untitled.md in the given folder" do
    assert_equal "work/2026/Untitled.md", Note.untitled("work/2026").path
    assert_equal "Untitled.md", Note.untitled(nil).path
    assert_equal "", Note.untitled("").content
    assert_not Note.untitled(nil).persisted?
  end

  test "save posts path and content and takes the id and version from the API" do
    api = use_fake_api(notes: [])
    note = Note.new(path: "work/plan.md", content: "# Plan\n")

    note.save

    assert_equal [ { path: "work/plan.md", content: "# Plan\n" } ], api.post_requests
    assert_equal "created-1", note.id
    assert_equal Time.utc(2026, 10, 1, 12), note.updated_at
  end

  test "save numbers an untitled note until the path is free" do
    use_fake_api(notes: [ { id: "1", path: "work/Untitled.md" }, { id: "2", path: "work/Untitled 1.md" } ])
    note = Note.untitled("work")

    note.save

    assert_equal "work/Untitled 2.md", note.path
    assert_equal "work/Untitled 2.md", Note.find(note.id).path
  end

  test "save does not renumber a path the user chose" do
    api = use_fake_api(notes: [ { id: "1", path: "plan.md" } ])

    assert_raises(MarkuppApi::DuplicatePath) { Note.new(path: "plan.md", content: "").save }
    assert_equal 1, api.post_requests.size
  end

  test "save gives up numbering at the limit, citing the last path tried" do
    taken = [ "Untitled.md", *(1..Note::UNTITLED_LIMIT).map { "Untitled #{it}.md" } ]
    use_fake_api(notes: taken.map.with_index { |path, index| { id: index.to_s, path: } })

    error = assert_raises(MarkuppApi::DuplicatePath) { Note.untitled(nil).save }
    assert_match "Untitled #{Note::UNTITLED_LIMIT}.md", error.message
    assert_match Note::UNTITLED_LIMIT.to_s, error.message
  end

  test "update raises Conflict when the note changed on the server" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)

    assert_raises(MarkuppApi::Conflict) { Note.new(id: "1", path: "a.md").update({ content: "mine" }) }
    assert_equal "theirs", Note.find("1").content
  end

  test "update renames and moves the note to the new path" do
    api = use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])

    assert Note.new(id: "1", path: "a.md").update({ path: "work/b.md", content: "x" })

    assert_equal "work/b.md", api.put_requests.sole[:path]
    assert_equal "work/b.md", Note.find("1").path
  end

  test "update returns false with a path error when another note has the path" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "y" } ])
    note = Note.new(id: "1", path: "a.md")

    assert_not note.update({ path: "b.md", content: "mine" })

    assert_equal [ "Path already taken" ], note.errors.full_messages_for(:path)
    assert_equal [ "b.md", "mine" ], [ note.path, note.content ]
    assert_equal "a.md", Note.find("1").path
  end

  test "update returns false with a path error when the API refuses the path" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "x" } ])
    note = Note.new(id: "1", path: "a.md")

    assert_not note.update({ path: " ", content: "x" })

    assert_equal [ "Path is invalid" ], note.errors.full_messages_for(:path)
  end
end
