require "test_helper"

# Configurações do cofre, seção Publication (ADR-0048): owner e admin publicam o cofre ativo com slug e título, copiam o
# link público e despublicam. O cofre ativo é o da primeira nota da árvore, como no Explorer e no grafo.
module Settings
  class PublicationsControllerTest < ActionDispatch::IntegrationTest
    ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
    CAIO = "caio@markupp.dev"
    PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
    ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: CAIO, password: PASSWORD } ].freeze
    ACME = { id: "tenant-1", name: "Acme research", members: { ANA => "owner", CAIO => "member" } }.freeze
    NOTES = [ { id: "plan", path: "plan.md", content: "" } ].freeze
    PUBLISHED = [ { vault_id: "vault-1", slug: "handbook", title: "Handbook" } ].freeze

    test "an unpublished vault shows the form to publish it, in the settings nav" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES)
      sign_in_as

      get settings_publication_path

      assert_response :success
      assert_select "main h1", "Publication"
      assert_select "main nav a[href=?][aria-current=page]", settings_publication_path, text: "Publication"
      assert_select "main form[action=?]", settings_publication_path do
        assert_select "input[type=hidden][name=vault_id][value=vault-1]"
        assert_select "input[name='publication[slug]'][required][pattern]"
        assert_select "input[name='publication[title]']"
        assert_select "button", "Publish"
      end
      assert_select "main", text: /Not published/
      assert_select "main [data-controller=copy-link]", count: 0
    end

    test "a published vault shows the public link to copy and the unpublish button" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES, publications: PUBLISHED)
      sign_in_as

      get settings_publication_path

      link = public_vault_url("handbook")
      assert_select "main input[name='publication[slug]'][value=handbook]"
      assert_select "main input[name='publication[title]'][value=Handbook]"
      assert_select "main [data-controller=copy-link][data-copy-link-text-value=?]", link do
        assert_select "a[href=?]", link, text: link
        assert_select "button[data-action='copy-link#copy']", "Copy link"
        assert_select "[role=status]"
      end
      assert_select "main form[action=?] button.btn-danger[data-turbo-confirm]", settings_publication_path, text: "Unpublish"
    end

    test "a member does not see the section and is sent back" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES)
      sign_in_as(CAIO)

      get settings_general_path
      assert_select "main nav a", text: "Publication", count: 0

      get settings_publication_path
      assert_redirected_to root_path
    end

    test "publishing sends the slug and the title and comes back with the link" do
      api = use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES)
      sign_in_as

      patch settings_publication_path, params: { vault_id: "vault-1", publication: { slug: "docs", title: "Docs" } }

      assert_redirected_to settings_publication_path
      assert_equal "Vault published at #{public_vault_url("docs")}.", flash[:notice]
      assert_equal [ "vault-1", "docs", "Docs" ], api.publications.sole.values_at("vault_id", "slug", "title")
    end

    test "a slug of another vault comes back on the field with 409" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES, publications: [ { vault_id: "vault-9", slug: "docs", title: "Other" } ])
      sign_in_as

      patch settings_publication_path, params: { vault_id: "vault-1", publication: { slug: "docs", title: "" } }

      assert_response :conflict
      assert_select "input[name='publication[slug]'][value=docs][aria-invalid=true]"
      assert_select "#publication_slug_error", text: /docs is already taken/
    end

    test "a slug out of the format comes back on the field with 422" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES)
      sign_in_as

      patch settings_publication_path, params: { vault_id: "vault-1", publication: { slug: "No Spaces", title: "" } }

      assert_response :unprocessable_content
      assert_select "#publication_slug_error", text: /lowercase letters, digits and hyphens/
    end

    test "unpublishing takes the vault out and says so" do
      api = use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES, publications: PUBLISHED)
      sign_in_as

      delete settings_publication_path, params: { vault_id: "vault-1" }

      assert_redirected_to settings_publication_path
      assert_equal "Vault unpublished. The public link no longer opens.", flash[:notice]
      assert_empty api.publications
    end

    test "unpublishing a vault that is no longer published says so" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ], notes: NOTES)
      sign_in_as

      delete settings_publication_path, params: { vault_id: "vault-1" }

      assert_redirected_to settings_publication_path
      assert_equal "The vault was not published.", flash[:alert]
    end

    test "a tenant without notes has no vault to publish yet" do
      use_fake_api(accounts: ACCOUNTS, tenants: [ ACME ])
      sign_in_as

      get settings_publication_path

      assert_response :success
      assert_select "main", text: /No vault yet/
      assert_select "main form[action=?]", settings_publication_path, count: 0
    end
  end
end
