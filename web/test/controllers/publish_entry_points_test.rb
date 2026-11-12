require "test_helper"

# Onde se publica (ADR-0048): a nota pelo cabeçalho dela (Alt+P), a pasta pelo menu da árvore e o cofre pela General
# dele. Tudo abre o mesmo diálogo do layout. A nota pública ganha a marca, na nota e na árvore.
class PublishEntryPointsTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  NOTES = [ { id: "start", path: "start.md", content: "# Start\n" }, { id: "deploy", path: "docs/deploy.md", content: "# Deploy\n" } ].freeze

  test "the layout has the publish dialog, which loads what goes public into its frame" do
    use_fake_api(notes: NOTES)
    sign_in_as

    get note_path("start")

    assert_select "dialog#publish_dialog[data-controller=publish-dialog] turbo-frame#publish_preview"
  end

  test "a writer publishes the open note from its header with Alt+P" do
    use_fake_api(notes: NOTES)
    sign_in_as

    get note_path("start")

    assert_select "a#publish_note[href=?][data-turbo-frame=publish_preview][aria-keyshortcuts='Alt+P']",
                  new_publication_path(kind: "note", note_id: "start"), text: /Publish/
  end

  test "a public note shows the mark with its public link instead of the publish button" do
    use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])
    sign_in_as

    get note_path("deploy")

    public_id = FakeMarkuppPublications.public_id_for("deploy")
    assert_select "a#publish_note", count: 0
    assert_select "a.public-mark[href=?]", public_note_path(public_id), text: /Public/
    assert_select "nav [role=treeitem][data-path='/docs/deploy'] .public-mark"
    assert_select "nav [role=treeitem][data-path='/start'] .public-mark", count: 0
  end

  test "a viewer gets no publish button" do
    use_fake_api(notes: NOTES, tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "viewer" } } ])
    sign_in_as

    get note_path("start")

    assert_select "a#publish_note", count: 0
    assert_select "#tree_menu [role=menuitem]", text: /Publish/, count: 0
  end

  test "the tree menu publishes a folder for the owner and admins, and a note for whoever writes" do
    use_fake_api(notes: NOTES)
    sign_in_as

    get note_path("start")

    assert_select "nav [data-note-tree-publish-url-value=?][data-note-tree-publish-folders-value=true]", new_publication_path
    assert_select "#tree_menu [role=menuitem][aria-keyshortcuts='Alt+Shift+P']", text: /Publish/
  end

  test "a member publishes notes from the tree, not folders" do
    use_fake_api(notes: NOTES, tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "member" } } ])
    sign_in_as

    get note_path("start")

    assert_select "nav [data-note-tree-publish-url-value][data-note-tree-publish-folders-value=false]"
  end

  test "the owner publishes the vault from its General settings, with v o" do
    use_fake_api(notes: NOTES)
    sign_in_as

    get settings_vault_path

    assert_select "#vault_publication a#publish_vault[href=?][data-turbo-frame=publish_preview][data-vault-action=o]",
                  new_publication_path(kind: "vault"), text: /Publish vault/
  end

  test "a member sees no publish vault" do
    use_fake_api(notes: NOTES, tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "member" } } ])
    sign_in_as

    get settings_vault_path

    assert_select "#vault_publication", count: 0
  end
end
