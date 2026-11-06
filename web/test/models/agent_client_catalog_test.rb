require "test_helper"

# Catálogo de clientes MCP: os modelos vêm do config/agent_clients.json, e cada cliente sai com os trechos e deeplinks
# que o próprio cliente aceita.
class AgentClientCatalogTest < ActiveSupport::TestCase
  DOCUMENT = JSON.parse(Rails.root.join("config/agent_clients.json").read)

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

  INSTANCE = "https://notas.exemplo.com.br/"
  MCP_URL = "https://notas.exemplo.com.br/mcp"

  def client_named(id) = AgentClientCatalog.new(DOCUMENT).clients_for(INSTANCE).find { it.id == id } || flunk("client #{id} missing")

  def snippet_text(client_id, snippet_id)
    snippet = client_named(client_id).snippets.find { it.id == snippet_id }
    snippet ? snippet.text : flunk("snippet #{client_id}/#{snippet_id} missing")
  end

  def deeplink_href(client_id) = client_named(client_id).deeplinks.first&.href || flunk("deeplink of #{client_id} missing")

  test "claude code registers the server over the http transport" do
    assert_equal "claude mcp add --transport http markupp #{MCP_URL}", snippet_text("claude-code", "cli")
  end

  test "codex registers the server by url and signs in with mcp login" do
    assert_equal "codex mcp add markupp --url #{MCP_URL}", snippet_text("codex", "cli")
    assert_equal "codex mcp login markupp", snippet_text("codex", "login")
    assert_equal %([mcp_servers.markupp]\nurl = "#{MCP_URL}"), snippet_text("codex", "config")
  end

  test "cursor gets a deeplink with the config in base64 and an mcp.json" do
    prefix = "cursor://anysphere.cursor-deeplink/mcp/install?name=markupp&config="
    href = deeplink_href("cursor")

    assert href.start_with?(prefix), href
    assert_equal({ "url" => MCP_URL }, JSON.parse(Base64.strict_decode64(CGI.unescape(href.delete_prefix(prefix)))))
    assert_equal({ "mcpServers" => { "markupp" => { "url" => MCP_URL } } }, JSON.parse(snippet_text("cursor", "config")))
  end

  test "vs code gets a deeplink with the json in the url, a command and an mcp.json" do
    prefix = "vscode:mcp/install?"
    href = deeplink_href("vscode")
    server = { "name" => "markupp", "type" => "http", "url" => MCP_URL }

    assert href.start_with?(prefix), href
    assert_equal server, JSON.parse(CGI.unescape(href.delete_prefix(prefix)))
    assert_equal "code --add-mcp '#{JSON.generate(server)}'", snippet_text("vscode", "cli")
    assert_equal({ "servers" => { "markupp" => { "type" => "http", "url" => MCP_URL } } }, JSON.parse(snippet_text("vscode", "config")))
  end

  test "claude desktop only gets the url of the custom connector" do
    assert_equal MCP_URL, snippet_text("claude-desktop", "url")
  end

  test "gemini cli uses httpUrl and windsurf uses serverUrl" do
    assert_equal "gemini mcp add --transport http markupp #{MCP_URL}", snippet_text("gemini-cli", "cli")
    assert_equal({ "mcpServers" => { "markupp" => { "httpUrl" => MCP_URL } } }, JSON.parse(snippet_text("gemini-cli", "config")))
    assert_equal({ "mcpServers" => { "markupp" => { "serverUrl" => MCP_URL } } }, JSON.parse(snippet_text("windsurf", "config")))
  end

  test "stdio signs in with the instance url, without /mcp, and runs npx" do
    assert_equal "npx -y markupp-mcp login https://notas.exemplo.com.br", snippet_text("stdio", "login")
    assert_equal({ "mcpServers" => { "markupp" => { "command" => "npx", "args" => %w[-y markupp-mcp] } } }, JSON.parse(snippet_text("stdio", "config")))
  end

  test "clients_for keeps the path of an instance outside the root and drops the trailing slashes" do
    claude_code = AgentClientCatalog.new(DOCUMENT).clients_for("http://localhost:8080/markupp//").first

    assert_equal "claude mcp add --transport http markupp http://localhost:8080/markupp/mcp", claude_code.snippets.first.text
  end

  test "clients_for refuses an instance url without a scheme, naming it" do
    error = assert_raises(ArgumentError) { AgentClientCatalog.new(DOCUMENT).clients_for("notas") }

    assert_match %r{url da instancia "notas", esperado http\(s\)://host}, error.message
  end

  test "load reads the catalog from a json file" do
    catalog = AgentClientCatalog.load(Rails.root.join("config/agent_clients.json"))

    assert_equal %w[claude-code codex cursor vscode claude-desktop gemini-cli windsurf stdio], catalog.clients_for("https://a.example").map(&:id)
  end
end
