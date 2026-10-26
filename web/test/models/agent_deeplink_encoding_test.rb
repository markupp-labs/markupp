require "test_helper"

# Codificação do json dos deeplinks, igual à do catálogo do servidor MCP (mcp/src/clients/link-encoding.ts).
class AgentDeeplinkEncodingTest < ActiveSupport::TestCase
  test "uri-json encodes the json as a strict url component" do
    assert_equal "%7B%22name%22%3A%22a%21%28%29%2A%27%22%7D", AgentDeeplinkEncoding.encode({ "name" => "a!()*'" }, "uri-json")
  end

  test "base64-json encodes the json in base64 and escapes its padding" do
    encoded = AgentDeeplinkEncoding.encode({ "url" => "https://a.b/?>>" }, "base64-json")

    assert_no_match %r{[+/=]}, encoded
    assert_equal '{"url":"https://a.b/?>>"}', Base64.strict_decode64(CGI.unescape(encoded))
  end

  test "an unknown encoding is refused with the expected ones" do
    error = assert_raises(ArgumentError) { AgentDeeplinkEncoding.encode({}, "rot13") }

    assert_match(/"rot13"/, error.message)
    assert_match(/base64-json/, error.message)
  end
end
