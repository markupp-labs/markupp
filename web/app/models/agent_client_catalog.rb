# Clientes MCP da página Agents, montados para a url pública da instância. Os modelos vêm do config/agent_clients.json,
# gerado pelo npm run clients:sync a partir do catálogo único do servidor MCP (mcp/src/clients/catalog.ts): aqui só se
# trocam os marcadores {{instanceUrl}} e {{mcpUrl}} e se codificam os deeplinks.
class AgentClientCatalog
  Client = Data.define(:id, :name, :docs_url, :snippets, :deeplinks)
  Snippet = Data.define(:id, :language, :path, :text)
  Deeplink = Data.define(:id, :href)

  def self.load(path) = new(JSON.parse(File.read(path)))

  def initialize(document)
    @templates = document.fetch("clients")
    @placeholders = document.fetch("placeholders")
  end

  def clients_for(instance_url)
    urls = urls_for(instance_url)
    @templates.map { build_client(it, urls) }
  end

  private
    # Valida a url e deriva a do MCP, que o Go serve em /mcp; a barra do fim sai, como no TypeScript.
    def urls_for(raw_url)
      uri = URI.parse(raw_url.to_s)
      raise URI::InvalidURIError unless uri.is_a?(URI::HTTP) && uri.host.present?

      instance_url = raw_url.sub(%r{/+\z}, "")
      { @placeholders.fetch("instanceUrl") => instance_url, @placeholders.fetch("mcpUrl") => "#{instance_url}/mcp" }
    rescue URI::InvalidURIError
      raise ArgumentError, "url da instancia #{raw_url.inspect}, esperado http(s)://host[/caminho]"
    end

    def build_client(template, urls)
      Client.new(id: template.fetch("id"), name: template.fetch("name"), docs_url: template.fetch("docsUrl"),
                 snippets: template.fetch("snippets").map { build_snippet(it, urls) },
                 deeplinks: template.fetch("deeplinks").map { build_deeplink(it, urls) })
    end

    def build_snippet(template, urls)
      Snippet.new(id: template.fetch("id"), language: template.fetch("language"), path: template["path"],
                  text: expand(template.fetch("template"), urls))
    end

    def build_deeplink(template, urls)
      payload = expand_json(template.fetch("payload"), urls)
      Deeplink.new(id: template.fetch("id"), href: template.fetch("base") + AgentDeeplinkEncoding.encode(payload, template.fetch("encoding")))
    end

    def expand(text, urls) = urls.reduce(text) { |result, (marker, url)| result.gsub(marker) { url } }

    def expand_json(value, urls)
      case value
      when String then expand(value, urls)
      when Array then value.map { expand_json(it, urls) }
      when Hash then value.transform_values { expand_json(it, urls) }
      else value
      end
    end
end
