require "test_helper"

# O painel se apresenta ao Go como o relay da leitura pública com o segredo de relay que o Go gera só para ele: o HTTP
# Basic com o usuário do painel e a senha codificados como form, do jeito que o Go confere.
class PublicRelayCredentialTest < ActiveSupport::TestCase
  PANEL_USER = "panel"
  SECRET = "a+b/c=" + ("x" * 32)
  SECRET_PATH = "/run/markupp-panel/secret"

  test "encodes the panel user and the secret as the Go server decodes them" do
    credential = PublicRelayCredential.new(secret: SECRET)

    user, password = Base64.strict_decode64(credential.authorization.delete_prefix("Basic ")).split(":", 2)
    assert_equal [ PANEL_USER, SECRET ], [ CGI.unescape(user), CGI.unescape(password) ]
  end

  test "reads the secret from the file the Go server writes, once it exists" do
    file = FakeSecretFile.new
    credential = PublicRelayCredential.new(secret_path: SECRET_PATH, file_reader: file)
    assert_nil credential.authorization

    file.write("#{SECRET}\n")

    assert_equal PublicRelayCredential.new(secret: SECRET).authorization, credential.authorization
  end

  test "an empty secret file is not written yet" do
    credential = PublicRelayCredential.new(secret_path: SECRET_PATH, file_reader: FakeSecretFile.new(" \n"))

    assert_nil credential.authorization
  end

  test "comes from the panel relay secret of the environment, without the MCP" do
    env = { "MARKUPP_PANEL_RELAY_SECRET" => SECRET }

    assert_equal PublicRelayCredential.new(secret: SECRET).authorization, PublicRelayCredential.from_env(env).authorization
  end

  test "comes from the panel relay secret file of the environment" do
    credential = PublicRelayCredential.from_env({ "MARKUPP_PANEL_RELAY_SECRET_PATH" => SECRET_PATH })

    assert_not_nil credential
  end

  test "the MCP secret is not the panel credential" do
    env = { "MARKUPP_MCP_URL" => "http://localhost:6275/mcp", "MARKUPP_MCP_RESOURCE_SECRET" => SECRET,
            "MARKUPP_MCP_SECRET_PATH" => "/run/markupp-mcp/secret" }

    assert_nil PublicRelayCredential.from_env(env)
  end
end
