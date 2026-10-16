require "test_helper"

class RefreshingMarkuppApiTest < ActiveSupport::TestCase
  setup do
    @fake_api = FakeMarkuppApi.new(notes: [ { id: "1", path: "a.md", content: "x" } ])
    @session = ActionController::TestSession.new
    @credential = PanelCredential.new(@session, ->(**credentials) { @fake_api.as(**credentials) })
    @credential.sign_in(@fake_api.post("/auth/login", email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD))
    @api = RefreshingMarkuppApi.new(@credential)
  end

  test "calls the API with the current access token" do
    assert_equal "a.md", @api.get("/notes").sole["path"]

    assert_equal [ "access-1" ], access_tokens_sent("GET /notes")
  end

  test "renews the token before calling once it has expired" do
    travel 901.seconds

    @api.get("/notes")

    assert_equal [ "access-2" ], access_tokens_sent("GET /notes")
    assert_equal "refresh-2", @credential.refresh_token
    assert_not @fake_api.refresh_token_valid?("refresh-1")
  end

  test "renews the token and repeats the call once when the API answers 401" do
    @fake_api.expire_access_tokens!

    @api.put("/notes/1", path: "a.md", content: "y")

    assert_equal [ "access-1", "access-2" ], access_tokens_sent("PUT /notes/1")
    assert_equal 1, @fake_api.put_requests.size
    assert_not @credential.access_expired?
  end

  test "raises SessionExpired with the original call when the refresh is refused" do
    @fake_api.expire_access_tokens!
    @fake_api.revoke_refresh_tokens!

    error = assert_raises(MarkuppApi::SessionExpired) { @api.delete("/notes/1") }

    assert_match "DELETE /notes/1", error.message
    assert_equal [ "access-1" ], access_tokens_sent("DELETE /notes/1")
  end

  test "forwards post and patch with their bodies" do
    @api.post("/notes", path: "b.md", content: "")

    assert_equal [ { path: "b.md", content: "" } ], @fake_api.post_requests
    error = assert_raises(ArgumentError) { @api.patch("/nowhere", role: "admin") }
    assert_match "PATCH /nowhere", error.message
  end

  private
    def access_tokens_sent(call)
      @fake_api.received_credentials.select { it[:call] == call }.map { it[:access_token] }
    end
end
