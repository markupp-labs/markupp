require "test_helper"

module Settings
  # Seção Publications (ADR-0048): tudo que está público no tenant, com o cofre escolhido na tela (o ativo já vem
  # marcado): o quê, quem, quando, a indexação e despublicar. Owner e admin mexem em tudo; quem escreve, nas notas.
  class PublicationsControllerTest < ActionDispatch::IntegrationTest
    EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
    NOTES = [ { id: "start", path: "start.md", content: "# Start\n" }, { id: "deploy", path: "docs/deploy.md", content: "# Deploy\n" },
              { id: "draft", path: "draft.md", content: "# Draft\n", vault_id: "vault-2" } ].freeze
    VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Notes" }, { id: "vault-2", tenant_id: "tenant-1", name: "Drafts" } ].freeze
    PUBLICATIONS = [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" }, { kind: "note", vault_id: "vault-1", note_id: "start" },
                     { kind: "note", vault_id: "vault-2", note_id: "draft", indexable: true } ].freeze

    setup do
      @api = use_fake_api(notes: NOTES, vaults: VAULTS, publications: PUBLICATIONS)
      sign_in_as
    end

    test "the section lists what is public in the active vault, with who and when" do
      get settings_publications_path

      assert_response :success
      assert_select "h1", "Publications"
      assert_select "form[action=?][method=get] select[name=vault_id][data-vault-action=p]", settings_publications_path do
        assert_select "option[selected][value=vault-1]", "Notes"
        assert_select "option[value=vault-2]", "Drafts"
        assert_select "option[value=all]", "All vaults"
      end
      assert_select "ul#publications[data-controller=context-menu] li[data-context-menu-target=item]", 2
      assert_select "#publications li", text: /Folder\s+docs/ do
        assert_select "*", text: /1 note/
        assert_select "*", text: /#{EMAIL}/
        assert_select "*", text: /Oct 20/
      end
      start = FakeMarkuppPublications.public_id_for("start")
      assert_select "#publications li a[href=?]", public_note_path("start-#{start}"), text: "start"
    end

    test "another vault, or all of them, comes from the selector" do
      get settings_publications_path(vault_id: "vault-2")
      assert_select "option[selected][value=vault-2]"
      assert_select "#publications li[data-context-menu-target=item]", 1

      get settings_publications_path(vault_id: "all")
      assert_select "#publications li[data-context-menu-target=item]", 3
      assert_select "#publications li", text: /Drafts/
    end

    test "indexing turns on and off for each publication" do
      id = @api.publications.first["id"]

      assert_select_toggle_for(id, "true", "Let search engines list it")
      patch settings_publication_path(id), params: { indexable: "true" }

      assert_redirected_to settings_publications_path(vault_id: "vault-1")
      assert_equal "Search engines may list docs now.", flash[:notice]
      assert @api.publications.first["indexable"]
    end

    test "unpublish asks first, and the notes go private again" do
      id = @api.publications.first["id"]
      get settings_publications_path
      assert_select "#publications form[action=?] button[data-context-menu-delete][data-turbo-confirm*=?]", settings_publication_path(id), "docs"

      delete settings_publication_path(id)

      assert_redirected_to settings_publications_path(vault_id: "vault-1")
      assert_equal "docs is private again. Its public links stopped working.", flash[:notice]
      assert_nil Note.find(@api, "deploy").public_id
    end

    test "unpublishing what is gone says so" do
      delete settings_publication_path("publication-99")

      assert_equal "That publication was already gone.", flash[:alert]
    end

    test "a member changes only the notes, and sees the folders without the buttons" do
      @api = use_fake_api(notes: NOTES, vaults: VAULTS, publications: PUBLICATIONS, tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "member" } } ])
      sign_in_as

      get settings_publications_path

      assert_select "#publications li", text: /Folder/ do
        assert_select "button", count: 0
      end
      assert_select "#publications li", text: /Note/ do
        assert_select "button", minimum: 2
      end
    end

    test "the navigation has the section for every member of the tenant, and v p opens it" do
      get settings_publications_path

      assert_select "nav a[href=?][aria-current=page]", settings_publications_path, text: "Publications"
      assert_equal settings_publications_path, JSON.parse(css_select("body").first["data-tenants-shortcut-vault-actions-value"])["p"]
    end

    test "a published vault says so in its General settings and points to the publications" do
      @api = use_fake_api(notes: NOTES, vaults: VAULTS, publications: [ { kind: "vault", vault_id: "vault-1" } ])
      sign_in_as

      get settings_vault_path

      assert_select "#vault_publication a#publish_vault", count: 0
      assert_select "#vault_publication", text: /The whole vault is public/
      assert_select "#vault_publication a[href=?]", settings_publications_path
    end

    private
      def assert_select_toggle_for(id, value, label)
        get settings_publications_path
        assert_select "#publications form[action=?] input[name=indexable][value=?] + button", settings_publication_path(id), value, text: label
      end
  end
end
