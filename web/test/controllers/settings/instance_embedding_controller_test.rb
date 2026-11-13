require "test_helper"

# Configurações da instância, seção Embedding: só o administrador da instância vê o grupo Instance e escolhe o provedor
# de embedding, a URL, o modelo e a chave, que o painel nunca mostra. Com a configuração de quem hospeda, o formulário
# fica travado; o estado do modelo local aparece embaixo.
module Settings
  class InstanceEmbeddingControllerTest < ActionDispatch::IntegrationTest
    ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
    ADMIN_ACCOUNTS = [ { email: ANA, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ].freeze

    def sign_in_admin(**embedding)
      fake = use_fake_api(accounts: ADMIN_ACCOUNTS, instance_embedding: embedding)
      sign_in_as
      fake
    end

    test "an instance admin sees the instance group, after the tenant group, with the embedding section" do
      sign_in_admin

      get settings_instance_embedding_path

      assert_response :success
      assert_select "main h1", "Embedding"
      groups = css_select("main nav p.text-xs").map(&:text)
      assert_equal "Instance", groups.last
      assert_select "main nav a[href=?][aria-current=page]", settings_instance_embedding_path, text: "Embedding"
    end

    test "an account that does not administer the instance has no instance group and is sent away" do
      use_fake_api
      sign_in_as

      get settings_account_path
      assert_select "main nav a[href=?]", settings_instance_embedding_path, count: 0
      assert_select "main nav p.text-xs", text: "Instance", count: 0

      get settings_instance_embedding_path
      assert_redirected_to root_path
    end

    test "the form shows the current choice and that a key is set, never the key" do
      sign_in_admin(provider: "openai-compat", url: "https://api.example/v1", model: "text-embedding-3-small", api_key: "sk-secret")

      get settings_instance_embedding_path

      assert_select "form#instance_embedding" do
        assert_select "input[type=radio][name=?][value=openai-compat][checked]", "embedding[provider]"
        assert_select "input[type=radio][name=?][value=local]", "embedding[provider]"
        assert_select "input[type=radio][name=?][value=none]", "embedding[provider]"
        assert_select "input[name=?][value=?]", "embedding[url]", "https://api.example/v1"
        assert_select "input[name=?][value=?]", "embedding[model]", "text-embedding-3-small"
        assert_select "input[type=password][name=?][autocomplete=off]:not([value])", "embedding[api_key]"
        assert_select "input[type=checkbox][name=?]", "embedding[clear_api_key]"
      end
      assert_select "main", text: /A key is set/
      assert_no_match(/sk-secret/, response.body)
    end

    test "saving sends the choice with the new key and says it was saved" do
      fake = sign_in_admin

      patch settings_instance_embedding_path, params: { embedding: { provider: "openai-compat", url: "https://api.example/v1", model: "m", api_key: "sk-new" } }

      assert_redirected_to settings_instance_embedding_path
      follow_redirect!
      assert_select "[role=status]", "Saved. The server switches to it within a minute."
      assert_equal({ provider: "openai-compat", url: "https://api.example/v1", model: "m", api_key: "sk-new" }, fake.instance_embedding_updates.last)
    end

    test "an empty key field keeps the stored key, and the remove box clears it" do
      fake = sign_in_admin(provider: "openai-compat", url: "https://api.example/v1", model: "m", api_key: "sk")

      patch settings_instance_embedding_path, params: { embedding: { provider: "openai-compat", url: "https://api.example/v1", model: "m", api_key: "" } }
      assert_not fake.instance_embedding_updates.last.key?(:api_key)

      patch settings_instance_embedding_path, params: { embedding: { provider: "local", api_key: "", clear_api_key: "1" } }
      assert_equal "", fake.instance_embedding_updates.last[:api_key]
    end

    test "a refused choice stays on the form with the reason" do
      sign_in_admin

      patch settings_instance_embedding_path, params: { embedding: { provider: "openai-compat", url: "", model: "m" } }

      assert_response :unprocessable_entity
      assert_select "main [role=alert]", "Check the provider: the remote endpoint needs an http or https URL and a model."
    end

    test "when whoever hosts the instance sets the provider, the form is locked and says by whom, without naming settings" do
      sign_in_admin(locked: true, host_provider: "local")

      get settings_instance_embedding_path

      assert_select "form#instance_embedding fieldset[disabled]"
      assert_select "main", text: /Set by whoever hosts this instance: Local model/
      assert_no_match(/MARKUPP_/, response.body)
    end

    test "the local model status shows downloading, ready or the error" do
      sign_in_admin(provider: "local", model_status: { state: "error", error: "sha256 mismatch", changed_at: "2026-11-10T12:00:00Z" })

      get settings_instance_embedding_path

      assert_select "#embedding_status", text: /Error/
      assert_select "#embedding_status", text: /sha256 mismatch/
    end

    test "the palette leads an instance admin to the embedding section" do
      sign_in_admin

      get settings_account_path

      assert_select "body[data-instance-admin]"
      assert_select "#command_palette_options [role=option][data-command-href=?]", settings_instance_embedding_path, text: /Instance embedding/
    end
  
    test "the instance group follows the admin list at each settings page, with one account read per page" do
      fake = use_fake_api
      sign_in_as
      fake.set_instance_admin!(ANA, true)

      get settings_account_path
      assert_select "main nav a[href=?]", settings_instance_embedding_path, text: "Embedding"
      assert_equal 1, fake.received_credentials.count { it[:call] == "GET /auth/me" } - 1, "one read at login, one for the page"

      fake.set_instance_admin!(ANA, false)
      get settings_account_path
      assert_select "main nav a[href=?]", settings_instance_embedding_path, count: 0
      get settings_instance_embedding_path
      assert_redirected_to root_path
    end
  end
end
