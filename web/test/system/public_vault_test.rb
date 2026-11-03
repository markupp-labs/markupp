require "application_system_test_case"

# Leitura do cofre publicado no navegador, sem conta (ADR-0048): a nota pelo mesmo renderizador do painel, os wikilinks
# e o ![[nota]] pela lista pública, e o grafo só leitura que abre a nota clicada.
class PublicVaultTest < ApplicationSystemTestCase
  NOTES = [
    { id: "start", path: "start.md", content: "# Start here\n\nRead [[guides/setup|the setup]] and ask [[secret]].\n\n![[guides/setup]]\n" },
    { id: "setup", path: "guides/setup.md", content: "# Setup\n\nRun **make**.\n" },
    { id: "secret", path: "secret.md", content: "# Secret\n", vault_id: "vault-2" }
  ].freeze
  NODES = [
    { id: "start", title: "start", path: "start.md", degree: 1, pagerank: 0.6, community: nil, unresolved: false },
    { id: "setup", title: "setup", path: "guides/setup.md", degree: 1, pagerank: 0.4, community: nil, unresolved: false }
  ].freeze
  GRAPH = { graph: { "vault-1" => { nodes: NODES, links: [ { source: 0, target: 1, type: "explicit", predicate: nil } ] } } }.freeze
  PUBLICATIONS = [ { vault_id: "vault-1", slug: "docs", title: "Markupp docs" } ].freeze

  setup do
    use_fake_api(notes: NOTES, graph: GRAPH, publications: PUBLICATIONS)
  end

  test "a visitor reads the note rendered, embeds the public note and follows a wikilink to it" do
    visit public_vault_path("docs", path: "start")

    assert_selector ".note-preview h1", text: "Start here"
    assert_selector ".note-preview .note-embed strong", text: "make"
    assert_selector ".note-preview .wikilink-missing", text: "secret"
    assert_no_text "# Start here"
    click_on "the setup"

    assert_current_path public_vault_path("docs", path: "guides/setup")
    assert_selector ".note-preview h1", text: "Setup"
    assert_selector "nav a[aria-current=page]", text: "setup"
  end

  test "the vault page draws the read only graph and a click on the list opens the note" do
    visit public_vault_path("docs")

    assert_selector "[data-graph-nodes='2'][data-graph-links='1']"
    find("[data-graph-view-target=list] a", text: "setup", visible: :all).execute_script("this.click()")

    assert_current_path public_vault_path("docs", path: "guides/setup")
  end

  test "an address with nothing published shows the friendly 404" do
    visit public_vault_path("nowhere", path: "start")

    assert_selector "h1", text: "404"
    assert_text "not published"
  end
end
