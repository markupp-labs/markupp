require "test_helper"

# Nota pública lida sem conta em /p/{title_slug}-{public_id} (ADR-0048): a árvore da publicação, os wikilinks só para
# notas públicas, o noindex salvo quando quem publicou liga a indexação, e o mesmo 404 para id inexistente e nota privada.
class PublicNotesControllerTest < ActionDispatch::IntegrationTest
  NOTES = [
    { id: "start", path: "docs/start.md", content: "# Start\n\nSee [[setup]] and [[secret]].\n" },
    { id: "setup", path: "docs/guides/setup.md", content: "# Setup\n" },
    { id: "secret", path: "secret.md", content: "# Secret\n" }
  ].freeze
  START = FakeMarkuppPublications.public_id_for("start")
  SETUP = FakeMarkuppPublications.public_id_for("setup")

  setup do
    @api = use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])
  end

  test "the note opens without an account, read only, with the tree of its publication" do
    get public_note_path("start-#{START}")

    assert_response :success
    assert_select "title", "start | docs"
    assert_select "body[data-area=published]"
    assert_select "header span", text: "docs"
    assert_select "header a", text: "docs", count: 0
    assert_select "nav a[href=?][aria-current=page]", public_note_path("start-#{START}"), text: "start"
    assert_select "nav details summary", text: "guides/"
    assert_select "nav details a[href=?]", public_note_path("setup-#{SETUP}"), text: "setup"
    assert_select "nav a", text: "secret", count: 0
    assert_select "main [data-controller=note-mode][data-mode=read] textarea[hidden][readonly]", text: /See \[\[setup\]\]/
    assert_select "main form", count: 0
  end

  test "the wikilinks resolve only to public notes, and the others read as plain text" do
    get public_note_path("start-#{START}")

    reading = css_select("main [data-controller=note-mode]").first
    assert_equal({ "setup" => public_note_path("setup-#{SETUP}") }, JSON.parse(reading["data-note-mode-wikilinks-value"]))
    assert_nil reading["data-note-mode-missing-note-value"]
  end

  test "the page says noindex until the publication turns indexing on" do
    get public_note_path("start-#{START}")
    assert_select "meta[name=robots][content=noindex]"

    Publication.change_indexing(@api, @api.publications.first["id"], true)
    get public_note_path("start-#{START}")
    assert_select "meta[name=robots]", count: 0
  end

  test "an address with an old title redirects for good to the current one" do
    get public_note_path("old-title-#{START}")

    assert_redirected_to public_note_path("start-#{START}")
    assert_response :moved_permanently
  end

  test "the id alone redirects to the address with the title" do
    get public_note_path(START)

    assert_redirected_to public_note_path("start-#{START}")
  end

  test "the embed of a public note comes in json with its content" do
    get public_note_path("setup-#{SETUP}", format: :json)

    assert_response :success
    assert_equal({ "title" => "setup", "content" => "# Setup\n" }, response.parsed_body)
  end

  test "a private note answers the same 404 as an id that does not exist" do
    [ FakeMarkuppPublications.public_id_for("secret"), "AbCdEf123456" ].each do |public_id|
      get public_note_path("secret-#{public_id}")

      assert_response :not_found
      assert_select "h1", text: /404/
      assert_select "nav", count: 0
    end
  end

  test "an address without a public id answers 404 without calling the API" do
    get "/p/start"

    assert_response :not_found
    assert_not @api.received_credentials.any? { it[:call].include?("/v1/public/") }
  end

  test "the visitor's IP goes to the API for the limit per IP" do
    get public_note_path("start-#{START}"), env: { "REMOTE_ADDR" => "203.0.113.9" }

    assert_includes @api.received_credentials, { access_token: nil, tenant_id: nil, forwarded_for: "203.0.113.9", call: "GET /v1/public/notes/#{START}" }
  end

  test "too many reads from the same IP answer 429 with a page that says to wait" do
    @api.limit_public_reads!

    get public_note_path("start-#{START}")

    assert_response :too_many_requests
    assert_select "h1", text: /429/
  end

  test "the API out of reach answers 502 in the public layout" do
    @api.cut_connection!

    get public_note_path("start-#{START}")

    assert_response :bad_gateway
    assert_select "body[data-area=published] h1", text: /502/
  end
end
