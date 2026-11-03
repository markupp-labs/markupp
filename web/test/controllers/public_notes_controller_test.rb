require "test_helper"

# Cofre publicado lido sem conta em /p/:slug (ADR-0048): a árvore das notas, a nota pelo caminho, com os wikilinks
# resolvidos pela lista pública, e 404 igual para slug inexistente e cofre não publicado.
class PublicNotesControllerTest < ActionDispatch::IntegrationTest
  NOTES = [
    { id: "start", path: "start.md", content: "# Start\n\nSee [[guides/setup]] and [[secret]].\n" },
    { id: "setup", path: "guides/setup.md", content: "# Setup\n" },
    { id: "secret", path: "secret.md", content: "# Secret\n", vault_id: "vault-2" }
  ].freeze
  PUBLICATIONS = [ { vault_id: "vault-1", slug: "docs", title: "Markupp docs" } ].freeze

  setup do
    @api = use_fake_api(notes: NOTES, publications: PUBLICATIONS)
  end

  test "the vault opens without an account, with the tree of its notes and the graph" do
    get public_vault_path("docs")

    assert_response :success
    assert_select "title", "Markupp docs"
    assert_select "body[data-area=published]"
    assert_select "header a[href=?]", public_vault_path("docs"), text: "Markupp docs"
    assert_select "nav a[href=?]", public_vault_path("docs", path: "start"), text: "start"
    assert_select "nav details summary", text: "guides/"
    assert_select "nav details a[href=?]", public_vault_path("docs", path: "guides/setup"), text: "setup"
    assert_select "nav a", text: "secret", count: 0
    assert_select "main [data-controller=graph-view][data-graph-view-url-value=?]", public_vault_graph_path("docs", format: :json)
  end

  test "a note opens by its path, read only, with the wikilinks of the public notes" do
    get public_vault_path("docs", path: "start")

    assert_response :success
    assert_select "title", "start | Markupp docs"
    assert_select "nav a[href=?][aria-current=page]", public_vault_path("docs", path: "start")
    assert_select "main [data-controller=note-mode][data-mode=read]" do |reading|
      targets = JSON.parse(reading.first["data-note-mode-wikilinks-value"])
      assert_equal({ "start" => public_vault_path("docs", path: "start"), "guides/setup" => public_vault_path("docs", path: "guides/setup") }, targets)
      assert_select "textarea[hidden][readonly][data-note-mode-target=source]", text: /See \[\[guides\/setup\]\]/
      assert_select "article.note-preview[data-note-mode-target=preview]"
    end
    assert_select "main form", count: 0
  end

  test "a note in a folder opens with its folder open in the tree" do
    get public_vault_path("docs", path: "guides/setup")

    assert_response :success
    assert_select "nav details[open] a[aria-current=page]", text: "setup"
  end

  test "the embed of a note comes in json with its path and content" do
    get public_vault_path("docs", path: "guides/setup.json"), headers: { "Accept" => "application/json" }

    assert_response :success
    assert_equal({ "path" => "guides/setup.md", "content" => "# Setup\n" }, response.parsed_body)
  end

  test "a vault that is not published answers the friendly 404, the same as a slug that does not exist" do
    [ "private", "nobody-here" ].each do |slug|
      get public_vault_path(slug, path: "start")

      assert_response :not_found
      assert_select "h1", text: /404/
      assert_select "main", text: /not published/
    end
  end

  test "a path with no note in the vault answers the friendly 404 with the way back to the vault" do
    get public_vault_path("docs", path: "secret")

    assert_response :not_found
    assert_select "main a[href=?]", public_vault_path("docs")
  end

  test "the 404 page takes the whole width, without the column of the tree" do
    get public_vault_path("private")

    assert_select "nav", count: 0
    assert_select "body > div[class*=grid-cols]", count: 0
  end

  test "a reader over the limit of the API gets the 429 page" do
    @api.limit_public_reads!

    get public_vault_path("docs")

    assert_response :too_many_requests
    assert_select "main", text: /Too many requests/
  end

  test "an API that does not answer gives the 502 page" do
    @api.cut_connection!

    get public_vault_path("docs")

    assert_response :bad_gateway
  end

  test "reads as the visitor, without a token and with the visitor IP for the limit of the API" do
    get public_vault_path("docs", path: "start"), env: { "REMOTE_ADDR" => "203.0.113.9" }

    public_calls = @api.received_credentials.select { it[:call].include?("/v1/public/") }
    assert_not_empty public_calls
    assert public_calls.all? { it[:access_token].nil? && it[:forwarded_for] == "203.0.113.9" }
  end

  test "a signed in reader reads the same page, without the panel around it" do
    sign_in_as

    get public_vault_path("docs", path: "start")

    assert_response :success
    assert_select "body[data-area=published]"
  end
end
