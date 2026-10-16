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

  test "get raises NotFound when the API refuses the id" do
    server = FakeHttpServer.new(status: "400 Bad Request", body: '{"error":"invalid_id","message":"id invalido"}')
    thread = server.serve_once

    assert_raises(MarkuppApi::NotFound) { MarkuppApi.new(server.url).get("/notes/abc") }
    thread.join
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

  test "post sends the body as JSON and parses the created note" do
    server = FakeHttpServer.new(status: "201 Created", body: '{"id":"9","path":"a.md"}')
    thread = server.serve_once

    assert_equal({ "id" => "9", "path" => "a.md" }, MarkuppApi.new(server.url).post("/notes", path: "a.md", content: ""))
    thread.join
    assert_equal [ "POST", "/notes" ], [ server.requested_method, server.requested_path ]
    assert_equal({ "path" => "a.md", "content" => "" }, JSON.parse(server.requested_body))
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

  test "put raises InvalidPath when the API refuses the path" do
    error = assert_put_raises(MarkuppApi::InvalidPath, '{"error":"invalid_path","message":"bad"}', status: "400 Bad Request")
    assert_match "400", error.message
    assert_kind_of MarkuppApi::Error, error
  end

  test "delete sends DELETE and accepts the empty 204 answer" do
    server = FakeHttpServer.new(status: "204 No Content", body: "")
    thread = server.serve_once

    assert_nil MarkuppApi.new(server.url).delete("/notes/1")
    thread.join
    assert_equal [ "DELETE", "/notes/1" ], [ server.requested_method, server.requested_path ]
  ensure
    server.close
  end

  test "sends the access token and the tenant in the headers" do
    server = FakeHttpServer.new(status: "200 OK", body: "[]")
    thread = server.serve_once

    MarkuppApi.new(server.url, access_token: "jwt-1", tenant_id: "tenant-7").get("/notes")
    thread.join
    assert_equal "Bearer jwt-1", server.requested_headers["authorization"]
    assert_equal "tenant-7", server.requested_headers["x-markupp-tenant"]
  ensure
    server.close
  end

  test "leaves out the credential headers when it has none" do
    server = FakeHttpServer.new(status: "200 OK", body: "{}")
    thread = server.serve_once

    MarkuppApi.new(server.url).post("/auth/login", email: "ana@markupp.dev", password: "x")
    thread.join
    assert_not server.requested_headers.key?("authorization")
    assert_not server.requested_headers.key?("x-markupp-tenant")
  ensure
    server.close
  end

  test "patch sends the body as JSON" do
    server = FakeHttpServer.new(status: "204 No Content", body: "")
    thread = server.serve_once

    assert_nil MarkuppApi.new(server.url, access_token: "jwt-1").patch("/users/9", role: "admin")
    thread.join
    assert_equal [ "PATCH", "/users/9" ], [ server.requested_method, server.requested_path ]
    assert_equal({ "role" => "admin" }, JSON.parse(server.requested_body))
  ensure
    server.close
  end

  test "maps the error codes of the auth routes to their classes" do
    {
      [ "401 Unauthorized", "unauthorized" ] => MarkuppApi::Unauthorized,
      [ "401 Unauthorized", "invalid_credentials" ] => MarkuppApi::InvalidCredentials,
      [ "401 Unauthorized", "invalid_refresh_token" ] => MarkuppApi::RefreshRejected,
      [ "400 Bad Request", "invalid_request" ] => MarkuppApi::InvalidRequest,
      [ "403 Forbidden", "local_auth_disabled" ] => MarkuppApi::LocalAuthDisabled
    }.each do |(status, code), error_class|
      error = assert_post_raises(error_class, %({"error":"#{code}","message":"recusado"}), status:)
      assert_match status.split.first, error.message
    end
  end

  test "an unknown 401 is Unauthorized and an unknown 403 is Forbidden" do
    assert_post_raises(MarkuppApi::Unauthorized, "not json", status: "401 Unauthorized")
    assert_post_raises(MarkuppApi::Forbidden, '{"error":"something_new"}', status: "403 Forbidden")
  end

  test "local auth disabled is a kind of Forbidden" do
    assert_operator MarkuppApi::LocalAuthDisabled, :<, MarkuppApi::Forbidden
  end

  test "maps the tenant error codes of the notes routes to TenantUnavailable" do
    [ [ "403 Forbidden", "tenant_forbidden" ], [ "403 Forbidden", "no_tenant" ], [ "400 Bad Request", "tenant_required" ] ].each do |status, code|
      error = assert_answer_raises(MarkuppApi::TenantUnavailable, %({"error":"#{code}","message":"recusado"}), status) { it.get("/notes") }
      assert_match code, error.message
    end
  end

  test "tenant unavailable is a kind of Forbidden" do
    assert_operator MarkuppApi::TenantUnavailable, :<, MarkuppApi::Forbidden
  end

  test "maps read_only to ReadOnly, a kind of Forbidden" do
    error = assert_put_raises(MarkuppApi::ReadOnly, '{"error":"read_only","message":"o papel viewer so le as notas"}', status: "403 Forbidden")
    assert_kind_of MarkuppApi::Forbidden, error
  end

  test "maps the error codes of the invite answers to their classes" do
    { [ "404 Not Found", "invite_not_found" ] => MarkuppApi::InviteNotFound,
      [ "409 Conflict", "already_member" ] => MarkuppApi::AlreadyMember }.each do |(status, code), error_class|
      assert_answer_raises(error_class, %({"error":"#{code}","message":"recusado"}), status) { it.post("/me/invites/1/accept", {}) }
    end
  end

  test "maps the error codes of the token routes to their classes" do
    { [ "400 Bad Request", "invalid_invite" ] => MarkuppApi::InvalidToken,
      [ "400 Bad Request", "invalid_token" ] => MarkuppApi::InvalidToken,
      [ "409 Conflict", "already_exists" ] => MarkuppApi::AccountExists }.each do |(status, code), error_class|
      assert_answer_raises(error_class, %({"error":"#{code}","message":"recusado"}), status) { it.post("/auth/invites/accept", token: "t", password: "p") }
    end
  end

  test "maps email_unavailable to EmailUnavailable" do
    assert_answer_raises(MarkuppApi::EmailUnavailable, '{"error":"email_unavailable","message":"sem email"}', "503 Service Unavailable") do
      it.post("/auth/password/forgot", email: "a")
    end
  end

  test "maps email_already_verified to EmailAlreadyVerified" do
    assert_answer_raises(MarkuppApi::EmailAlreadyVerified, '{"error":"email_already_verified","message":"ja verificado"}', "409 Conflict") do
      it.post("/auth/email/verify", token: "t")
    end
  end

  test "maps the error codes of the member routes to their classes" do
    { [ "404 Not Found", "member_not_found" ] => MarkuppApi::MemberNotFound,
      [ "409 Conflict", "last_owner" ] => MarkuppApi::LastOwner,
      [ "403 Forbidden", "role_above_own" ] => MarkuppApi::RoleAboveOwn }.each do |(status, code), error_class|
      assert_answer_raises(error_class, %({"error":"#{code}","message":"recusado"}), status) { it.patch("/users/1", role: "owner") }
    end
  end

  test "member not found is a kind of NotFound and role above own a kind of Forbidden" do
    assert_operator MarkuppApi::MemberNotFound, :<, MarkuppApi::NotFound
    assert_operator MarkuppApi::RoleAboveOwn, :<, MarkuppApi::Forbidden
  end

  test "an error names its reason after its class, for the i18n keys" do
    assert_equal "role_above_own", MarkuppApi::RoleAboveOwn.new.reason
    assert_equal "local_auth_disabled", MarkuppApi::LocalAuthDisabled.new.reason
  end

  test "path joins the segments, encoding each one" do
    assert_equal "/me/invites/a%2Fb%20c/accept", MarkuppApi.path("me", "invites", "a/b c", "accept")
  end

  private
    def assert_post_raises(error_class, body, status:)
      assert_answer_raises(error_class, body, status) { it.post("/auth/login", email: "a", password: "b") }
    end

    def assert_put_raises(error_class, body, status: "409 Conflict")
      assert_answer_raises(error_class, body, status) { it.put("/notes/1", content: "x") }
    end

    def assert_answer_raises(error_class, body, status)
      server = FakeHttpServer.new(status:, body:)
      thread = server.serve_once
      error = assert_raises(error_class) { yield MarkuppApi.new(server.url) }
      thread.join
      error
    ensure
      server.close
    end
end
