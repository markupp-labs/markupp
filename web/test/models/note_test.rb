require "test_helper"

class NoteTest < ActiveSupport::TestCase
  test "all builds notes from the API listing" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "work/plan.md", content: "# Plan", updated_at: "2026-09-30T12:00:00Z" } ])

    note = Note.all(api).sole
    assert_equal "1", note.id
    assert_equal "work/plan.md", note.path
    assert_equal Time.utc(2026, 9, 30, 12), note.updated_at
  end

  test "find builds the note with the given id" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "# B\n" } ])

    note = Note.find(api, "2")
    assert_equal "b.md", note.path
    assert_equal "# B\n", note.content
  end

  test "find raises NotFound for an unknown id" do
    api = FakeMarkuppApi.new(notes: [])

    assert_raises(MarkuppApi::NotFound) { Note.find(api, "missing") }
  end

  test "search builds notes from the matches without content" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "work/plan.md", content: "Ship it", updated_at: "2026-09-30T12:00:00Z" },
                          { id: "2", path: "inbox.md", content: "nothing" } ])

    note = Note.search(api, "SHIP").sole
    assert_equal [ "1", "work/plan.md", Time.utc(2026, 9, 30, 12) ], [ note.id, note.path, note.updated_at ]
    assert_nil note.content
  end

  test "search encodes the query" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "a & b?" } ])

    assert_equal [ "1" ], Note.search(api, "a & b?").map(&:id)
  end

  test "search asks for at most 20 matches" do
    api = FakeMarkuppApi.new(notes: Array.new(21) { |index| { id: index.to_s, path: "#{index}.md", content: "match" } })

    assert_equal 20, Note.search(api, "match").size
  end

  test "search with a blank query is empty without calling the API" do
    api = FakeMarkuppApi.new(unreachable: true)

    assert_empty Note.search(api, "")
    assert_empty Note.search(api, nil)
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
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "old" } ])
    note = Note.new(id: "1", path: "a.md", updated_at: "2026-09-30T12:00:00.123456Z")

    note.update(api, { content: "new" })

    assert_equal [ { path: "a.md", content: "new", lastModifiedAt: "2026-09-30T12:00:00.123Z", force: false } ], api.put_requests
    assert_equal "new", Note.find(api, "1").content
  end

  test "update with force overwrites a newer version" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "old" } ], conflict: true)

    Note.new(id: "1", path: "a.md", updated_at: Time.utc(2026)).update(api, { content: "mine" }, force: true)

    assert api.put_requests.sole[:force]
    assert_equal "mine", Note.find(api, "1").content
  end

  test "save posts path and content and takes the id and version from the API" do
    api = FakeMarkuppApi.new(notes: [])
    note = Note.new(path: "work/plan.md", content: "# Plan\n")

    note.save(api)

    assert_equal [ { path: "work/plan.md", content: "# Plan\n" } ], api.post_requests
    assert_equal "created-1", note.id
    assert_equal Time.utc(2026, 10, 1, 12), note.updated_at
  end

  test "save flags a taken path after a single try" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "plan.md" } ])

    note = Note.new(path: "plan.md", content: "")

    assert_not note.save(api)
    assert note.errors.of_kind?(:path, :taken)
    assert_equal 1, api.post_requests.size
  end

  test "update raises Conflict when the note changed on the server" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "theirs" } ], conflict: true)

    assert_raises(MarkuppApi::Conflict) { Note.new(id: "1", path: "a.md").update(api, { content: "mine" }) }
    assert_equal "theirs", Note.find(api, "1").content
  end

  test "update renames and moves the note to the new path" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "x" } ])

    assert Note.new(id: "1", path: "a.md").update(api, { path: "work/b.md", content: "x" })

    assert_equal "work/b.md", api.put_requests.sole[:path]
    assert_equal "work/b.md", Note.find(api, "1").path
  end

  test "update returns false with a path error when another note has the path" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "x" }, { id: "2", path: "b.md", content: "y" } ])
    note = Note.new(id: "1", path: "a.md")

    assert_not note.update(api, { path: "b.md", content: "mine" })

    assert_equal [ "Path already taken" ], note.errors.full_messages_for(:path)
    assert_equal [ "b.md", "mine" ], [ note.path, note.content ]
    assert_equal "a.md", Note.find(api, "1").path
  end

  test "update returns false with a path error when the API refuses the path" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "x" } ])
    note = Note.new(id: "1", path: "a.md")

    assert_not note.update(api, { path: " ", content: "x" })

    assert_equal [ "Path is invalid" ], note.errors.full_messages_for(:path)
  end

  test "destroy deletes the note on the server" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "x" } ])

    Note.new(id: "1").destroy(api)

    assert_raises(MarkuppApi::NotFound) { Note.find(api, "1") }
  end

  test "destroy of a note already deleted does not raise" do
    api = FakeMarkuppApi.new(notes: [])

    assert_nothing_raised { Note.new(id: "gone").destroy(api) }
  end

  test "restore_line_breaks brings back CRLF when the note used it" do
    assert_equal "a\r\nb\r\n", Note.restore_line_breaks("a\nb\n", crlf: true)
    assert_equal "a\r\nb\r\n", Note.restore_line_breaks("a\r\nb\r\n", crlf: true)
  end

  test "restore_line_breaks keeps LF when the note used it, even after a native form submit" do
    assert_equal "a\nb\n", Note.restore_line_breaks("a\r\nb\r\n", crlf: false)
    assert_equal "a\nb\n", Note.restore_line_breaks("a\nb\n", crlf: false)
  end

  test "crlf? tells whether the content uses CRLF" do
    assert Note.new(content: "a\r\nb").crlf?
    assert_not Note.new(content: "a\nb").crlf?
  end

  test "title is the name without .md, folder the path before it" do
    note = Note.new(path: "work/2026/Plan.v2.md")

    assert_equal "Plan.v2", note.title
    assert_equal "work/2026", note.folder
    assert note.markdown?
    assert_equal [ "README", "", false ], Note.new(path: "README").then { [ it.title, it.folder, it.markdown? ] }
  end

  test "compose_path joins folder and title and adds .md back" do
    assert_equal "work/2026/Plan.md", Note.compose_path(folder: "work/2026", title: "Plan")
    assert_equal "Plan.md", Note.compose_path(folder: "", title: " Plan ")
    assert_equal "README", Note.compose_path(folder: nil, title: "README", markdown: false)
  end

  test "compose_path reads the folder like a unix path, with or without the slashes around it" do
    assert_equal "work/2026/Plan.md", Note.compose_path(folder: "/work/2026/", title: "Plan")
    assert_equal "Plan.md", Note.compose_path(folder: "/", title: "Plan")
  end

  test "compose_path leaves the name empty for a blank title" do
    assert_equal "work/.md", Note.compose_path(folder: "work", title: "  ")
    assert_equal "work/", Note.compose_path(folder: "work", title: "", markdown: false)
  end

  test "a path ending in a slash has an empty title and keeps the folder" do
    assert_equal [ "", "work", false ], Note.new(path: "work/").then { [ it.title, it.folder, it.markdown? ] }
  end

  test "update refuses a blank title without calling the API" do
    api = FakeMarkuppApi.new(notes: [ { id: "1", path: "work/a.md", content: "x" } ])
    note = Note.new(id: "1")

    assert_not note.update(api, { path: "work/.md", content: "mine" })
    assert_not note.update(api, { path: "work/", content: "mine" })
    assert note.errors.of_kind?(:title, :blank)
    assert_empty api.put_requests
    assert_equal "work/a.md", Note.find(api, "1").path
  end
end
