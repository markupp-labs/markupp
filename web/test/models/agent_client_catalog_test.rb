require "test_helper"

# Catálogo de clientes MCP: os modelos vêm do config/agent_clients.json, que o npm run clients:sync gera a partir do
# catálogo do servidor MCP. O exemplo montado lá é o contrato: o Ruby tem de montar igual ao TypeScript.
class AgentClientCatalogTest < ActiveSupport::TestCase
  DOCUMENT = JSON.parse(Rails.root.join("config/agent_clients.json").read)

  # O cliente montado no formato do clients.json, para comparar com o exemplo.
  def as_catalog_json(client)
    { "id" => client.id, "name" => client.name, "docsUrl" => client.docs_url,
      "snippets" => client.snippets.map { { "id" => it.id, "language" => it.language, "path" => it.path, "text" => it.text }.compact },
      "deeplinks" => client.deeplinks.map { { "id" => it.id, "href" => it.href } } }
  end

  test "clients_for builds the same clients as the MCP catalog example" do
    catalog = AgentClientCatalog.new(DOCUMENT)
    example = DOCUMENT.fetch("example")

    assert_equal example.fetch("clients"), catalog.clients_for(example.fetch("instanceUrl")).map { as_catalog_json(it) }
  end

  test "clients_for puts the MCP url, under the instance, in the snippets" do
    claude_code = AgentClientCatalog.new(DOCUMENT).clients_for("http://localhost:8080/").first

    assert_equal "claude mcp add --transport http markupp http://localhost:8080/mcp", claude_code.snippets.first.text
  end

  test "clients_for puts the MCP url it is given in the snippets" do
    claude_code = AgentClientCatalog.new(DOCUMENT).clients_for("https://a.example", mcp_url: "https://mcp.a.example/mcp/").first

    assert_equal "claude mcp add --transport http markupp https://mcp.a.example/mcp", claude_code.snippets.first.text
  end

  test "clients_for refuses an MCP url that is not http, naming it" do
    error = assert_raises(ArgumentError) { AgentClientCatalog.new(DOCUMENT).clients_for("https://a.example", mcp_url: "mcp") }

    assert_match %r{url do MCP "mcp"}, error.message
    assert_match %r{http\(s\)://host}, error.message
  end

  test "clients_for refuses an url that is not http, naming it" do
    error = assert_raises(ArgumentError) { AgentClientCatalog.new(DOCUMENT).clients_for("ftp://notas.exemplo.com.br") }

    assert_match %r{"ftp://notas.exemplo.com.br"}, error.message
    assert_match %r{http\(s\)://host}, error.message
  end

  test "load reads the catalog from a json file" do
    catalog = AgentClientCatalog.load(Rails.root.join("config/agent_clients.json"))

    assert_equal %w[claude-code codex cursor vscode claude-desktop gemini-cli windsurf stdio], catalog.clients_for("https://a.example").map(&:id)
  end
end
