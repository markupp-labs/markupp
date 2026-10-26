# Como o json de um deeplink entra na url, igual ao catálogo do servidor MCP (mcp/src/clients/link-encoding.ts): em
# base64 (Cursor) ou direto (VS Code), e nos dois casos codificado como componente estrito de url.
module AgentDeeplinkEncoding
  ENCODINGS = %w[base64-json uri-json].freeze

  # JSON.generate e não to_json: o to_json do Rails escapa < > & e o deeplink sairia diferente do TypeScript.
  def self.encode(payload, encoding)
    json = JSON.generate(payload)
    case encoding
    when "uri-json" then ERB::Util.url_encode(json)
    when "base64-json" then ERB::Util.url_encode(Base64.strict_encode64(json))
    else raise ArgumentError, "codificacao de deeplink #{encoding.inspect}, esperado uma de #{ENCODINGS.join(', ')}"
    end
  end
end
