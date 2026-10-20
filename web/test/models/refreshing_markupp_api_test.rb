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

  test "conditional_get passes the ETag on with the current access token" do
    @fake_api.change_graph!("vault-1", { nodes: [], links: [] }, etag: 'W/"3"')

    assert_nil @api.conditional_get("/v1/vaults/vault-1/graph", etag: 'W/"3"')

    assert_equal [ 'W/"3"' ], @fake_api.received_etags
    assert_equal [ "access-1" ], access_tokens_sent("GET /v1/vaults/vault-1/graph")
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

  test "two renewals of the same refresh token call /auth/refresh once and share the new pair" do
    renewed_pairs = ActiveSupport::Cache::MemoryStore.new
    twin = PanelCredential.new(ActionController::TestSession.new(@session.to_hash), ->(**credentials) { @fake_api.as(**credentials) })
    @fake_api.hold_refreshes_together!(2, timeout: 0.3)
    travel 901.seconds

    [ @credential, twin ].map { |credential| Thread.new { renew_and_list_notes(credential, renewed_pairs) } }.each(&:join)

    assert_equal 1, @fake_api.received_credentials.count { it[:call] == "POST /auth/refresh" }
    assert_equal [ "refresh-2", "refresh-2" ], [ @credential.refresh_token, twin.refresh_token ]
    assert renewed_pairs.exist?("panel/renewed_pair/#{Digest::SHA256.hexdigest("refresh-1")}")
  end

  test "the renewed pair goes to the cache sealed, with no token in plain text" do
    renewed_pairs = ActiveSupport::Cache::MemoryStore.new
    travel 901.seconds

    RefreshingMarkuppApi.new(@credential, renewed_pairs:).get("/notes")

    sealed = renewed_pairs.read(renewed_pair_key("refresh-1")).to_s
    [ "access-1", "refresh-1", "access-2", "refresh-2" ].each { assert_not_includes sealed, it }
  end

  test "a forged or corrupted value in the cache is ignored and the token is renewed" do
    renewed_pairs = CacheWithoutExpiry.new
    forged_pair = { "access_token" => "forged", "refresh_token" => "forged", "expires_in" => 900 }

    [ forged_pair, "Zm9yZ2Vk--Zm9yZ2Vk--Zm9yZ2Vk" ].each.with_index(2) do |value, generation|
      renewed_pairs.write(renewed_pair_key(@credential.refresh_token), value)
      travel 901.seconds
      renew_and_list_notes(@credential, renewed_pairs)
      assert_equal "refresh-#{generation}", @credential.refresh_token
    end
  end

  test "a remembered pair past its embedded expiry is ignored even if the cache still holds it" do
    renewed_pairs = CacheWithoutExpiry.new
    twin = PanelCredential.new(ActionController::TestSession.new(@session.to_hash), ->(**credentials) { @fake_api.as(**credentials) })
    travel 901.seconds
    RefreshingMarkuppApi.new(@credential, renewed_pairs:).get("/notes")

    travel RefreshingMarkuppApi::RENEWED_PAIR_TTL + 1.second
    renew_and_list_notes(twin, renewed_pairs)

    assert_equal 2, @fake_api.received_credentials.count { it[:call] == "POST /auth/refresh" }
    assert_not_equal "refresh-2", twin.refresh_token
  end

  private
    def renewed_pair_key(refresh_token) = "panel/renewed_pair/#{Digest::SHA256.hexdigest(refresh_token)}"
    # A segunda renovação com o refresh já usado é recusada; o teste confere a contagem de chamadas, não a exceção.
    def renew_and_list_notes(credential, renewed_pairs)
      RefreshingMarkuppApi.new(credential, renewed_pairs:).get("/notes")
    rescue MarkuppApi::SessionExpired
      nil
    end

    def access_tokens_sent(call)
      @fake_api.received_credentials.select { it[:call] == call }.map { it[:access_token] }
    end
end
