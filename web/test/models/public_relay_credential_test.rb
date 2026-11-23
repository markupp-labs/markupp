require "test_helper"

# O painel se apresenta ao Go como o relay do MCP na leitura pública (ADR-0050): o HTTP Basic do resource, com usuário e
# senha codificados como form, do jeito que o Go confere.
class PublicRelayCredentialTest < ActiveSupport::TestCase
  RESOURCE = "http://localhost:6275/mcp"
  SECRET = "a+b/c=" + ("x" * 32)
  SECRET_PATH = "/run/markupp-mcp/secret"

  test "encodes the resource and the secret as the Go server decodes them" do
    credential = PublicRelayCredential.new(RESOURCE, secret: SECRET)

    user, password = Base64.strict_decode64(credential.authorization.delete_prefix("Basic ")).split(":", 2)
    assert_equal [ RESOURCE, SECRET ], [ CGI.unescape(user), CGI.unescape(password) ]
  end

  test "reads the secret from the file the Go server writes, once it exists" do
    file = FakeSecretFile.new
    credential = PublicRelayCredential.new(RESOURCE, secret_path: SECRET_PATH, file_reader: file)
    assert_nil credential.authorization

    file.write("#{SECRET}\n")

    assert_equal PublicRelayCredential.new(RESOURCE, secret: SECRET).authorization, credential.authorization
  end

  test "an empty secret file is not written yet" do
    credential = PublicRelayCredential.new(RESOURCE, secret_path: SECRET_PATH, file_reader: FakeSecretFile.new(" \n"))

    assert_nil credential.authorization
  end

  test "comes from the MCP url and the secret of the environment" do
    env = { "MARKUPP_MCP_URL" => RESOURCE, "MARKUPP_MCP_RESOURCE_SECRET" => SECRET }

    assert_equal PublicRelayCredential.new(RESOURCE, secret: SECRET).authorization, PublicRelayCredential.from_env(env).authorization
  end

  test "there is no credential when the instance does not serve the MCP" do
    assert_nil PublicRelayCredential.from_env({ "MARKUPP_MCP_RESOURCE_SECRET" => SECRET })
  end
end
