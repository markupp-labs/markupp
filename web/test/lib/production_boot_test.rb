require "test_helper"
require "open3"

# Os testes rodam no ambiente test, e nada além do build da imagem passa pelo
# production.rb. Este teste sobe o ambiente de produção num processo à parte, como o
# assets:precompile do Dockerfile faz.
class ProductionBootTest < ActiveSupport::TestCase
  test "the production environment boots like the image build does" do
    env = { "RAILS_ENV" => "production", "SECRET_KEY_BASE_DUMMY" => "1" }
    output, status = Open3.capture2e(env, Rails.root.join("bin/rails").to_s, "runner", "print :ok", chdir: Rails.root.to_s)

    assert status.success?, "production não sobe: #{output}"
  end
end
