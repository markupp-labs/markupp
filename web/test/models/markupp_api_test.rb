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

  test "get raises when the API cannot be reached" do
    server = FakeHttpServer.new(status: "200 OK", body: "[]")
    url = server.url
    server.close

    error = assert_raises(MarkuppApi::Error) { MarkuppApi.new(url).get("/notes") }
    assert_match url, error.message
  end
end
