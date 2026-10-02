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

  test "update raises Conflict when the note changed on the server" do
    use_fake_api(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)

    assert_raises(MarkuppApi::Conflict) { Note.new(id: "1", path: "a.md").update({ content: "mine" }) }
    assert_equal "theirs", Note.find("1").content
  end
end
