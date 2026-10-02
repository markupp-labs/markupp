require "test_helper"

class MarkuppApiTest < ActiveSupport::TestCase
  test "get parses the JSON body of a successful response" do
    server = FakeHttpServer.new(status: "200 OK", body: '[{"path":"a.md"}]')
    thread = server.serve_once

    assert_equal [ { "path" => "a.md" } ], MarkuppApi.new(server.url).get("/notes")
    thread.join
    assert_equal "/notes", server.requested_path
  ensure
    server.close
  end

  test "get raises with status and body when the API answers an error" do
    server = FakeHttpServer.new(status: "500 Internal Server Error", body: '{"error":"boom"}')
    thread = server.serve_once

    error = assert_raises(MarkuppApi::Error) { MarkuppApi.new(server.url).get("/notes") }
    thread.join
    assert_match "500", error.message
    assert_match "boom", error.message
  ensure
    server.close
  end

  test "get raises NotFound when the API answers 404" do
    server = FakeHttpServer.new(status: "404 Not Found", body: '{"error":"not_found"}')
    thread = server.serve_once

    error = assert_raises(MarkuppApi::NotFound) { MarkuppApi.new(server.url).get("/notes/42") }
    thread.join
    assert_match "/notes/42", error.message
    assert_kind_of MarkuppApi::Error, error
  ensure
    server.close
  end

  test "get raises when the API cannot be reached" do
    server = FakeHttpServer.new(status: "200 OK", body: "[]")
    url = server.url
    server.close

    error = assert_raises(MarkuppApi::Error) { MarkuppApi.new(url).get("/notes") }
    assert_match url, error.message
  end

  test "put sends the body as JSON and parses the answer" do
    server = FakeHttpServer.new(status: "200 OK", body: '{"id":"1","content":"v2"}')
    thread = server.serve_once

    assert_equal({ "id" => "1", "content" => "v2" }, MarkuppApi.new(server.url).put("/notes/1", content: "v2", force: false))
    thread.join
    assert_equal [ "PUT", "/notes/1" ], [ server.requested_method, server.requested_path ]
    assert_equal({ "content" => "v2", "force" => false }, JSON.parse(server.requested_body))
  ensure
    server.close
  end

  test "put raises Conflict when the note changed since it was read" do
    error = assert_put_raises(MarkuppApi::Conflict, '{"error":"conflict","message":"changed"}')
    assert_match "409", error.message
    assert_kind_of MarkuppApi::Error, error
  end

  test "put raises DuplicatePath when another note has the path" do
    error = assert_put_raises(MarkuppApi::DuplicatePath, '{"error":"duplicate_path","message":"taken"}')
    assert_kind_of MarkuppApi::Error, error
  end

  test "put raises a plain Error for a 409 it does not know" do
    error = assert_put_raises(MarkuppApi::Error, "not json")
    assert_not_kind_of MarkuppApi::Conflict, error
    assert_not_kind_of MarkuppApi::DuplicatePath, error
  end

  private
    def assert_put_raises(error_class, body)
      server = FakeHttpServer.new(status: "409 Conflict", body:)
      thread = server.serve_once
      error = assert_raises(error_class) { MarkuppApi.new(server.url).put("/notes/1", content: "x") }
      thread.join
      error
    ensure
      server.close
    end
end
