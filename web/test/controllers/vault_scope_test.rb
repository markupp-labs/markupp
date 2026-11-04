require "test_helper"

# Cofre ativo na sessão: a árvore, o Explorer, a busca, o grafo e a nota nova agem nele; trocar de tenant o esquece.
class VaultScopeTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TWO_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                  { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ].freeze
  VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ EMAIL ] },
             { id: "vault-2", tenant_id: "tenant-1", name: "Drafts", members: [ EMAIL ] },
             { id: "vault-3", tenant_id: "tenant-2", name: "Lab", members: [ EMAIL ] } ].freeze
  NOTES = [ { id: "1", path: "plan.md", content: "ship it", vault_id: "vault-1", pagerank: 0.5 },
            { id: "2", path: "draft.md", content: "ship later", vault_id: "vault-2", pagerank: 0.4 } ].freeze

  setup do
    @api = use_fake_api(notes: NOTES, vaults: VAULTS)
    sign_in_as
  end

  test "the tree shows the notes of the first vault when the session has none" do
    get root_path

    assert_select "nav [role=treeitem]", text: "└── plan"
    assert_select "nav [role=treeitem]", text: "└── draft", count: 0
  end

  test "the tree lists every page of the vault notes" do
    many = (1..105).map { { id: "n#{it}", path: "n#{it.to_s.rjust(3, "0")}.md", content: "", vault_id: "vault-1" } }
    use_fake_api(notes: many, vaults: VAULTS)
    sign_in_as

    get root_path

    assert_select "nav [role=tree] [role=treeitem]", count: 105
  end

  test "switching the vault shows its notes and keeps it for the next pages" do
    patch current_vault_path, params: { vault_id: "vault-2" }
    assert_redirected_to root_path

    get root_path
    assert_select "nav [role=treeitem]", text: "└── draft"
    assert_select "nav [role=treeitem]", text: "└── plan", count: 0
  end

  test "switching to a vault outside the list keeps the active one and says so" do
    patch current_vault_path, params: { vault_id: "vault-9" }

    assert_redirected_to root_path
    assert_equal "You no longer have access to that vault.", flash[:alert]
    get root_path
    assert_select "nav [role=treeitem]", text: "└── plan"
  end

  test "switching the tenant forgets the vault of the other tenant" do
    use_fake_api(notes: NOTES + [ { id: "3", path: "lab.md", content: "", vault_id: "vault-3" } ], vaults: VAULTS, tenants: TWO_TENANTS)
    sign_in_as
    patch current_tenant_path, params: { tenant_id: "tenant-1" }
    patch current_vault_path, params: { vault_id: "vault-2" }

    patch current_tenant_path, params: { tenant_id: "tenant-2" }
    get root_path

    assert_select "nav [role=treeitem]", text: "└── lab"
  end

  # Link direto ou wikilink para nota de outro cofre: o painel segue a nota, e a árvore, o Explorer e o grafo com ele.
  test "opening a note of another vault of the account makes that vault the active one" do
    get note_path("2")

    assert_response :success
    assert_select "button[popovertarget=vault_switcher]", text: "Drafts"
    assert_select "nav [role=treeitem]", text: "└── draft"
    assert_select "nav [role=treeitem]", text: "└── plan", count: 0
    get explorer_path(mode: "central")
    assert_select "turbo-frame#explorer_list a", text: /draft/
    get graph_path
    assert_select "[data-graph-view-url-value=?]", graph_path(format: :json, vault: "vault-2")
  end

  test "opening a note of a vault the account is not a member of keeps the active vault" do
    use_fake_api(notes: NOTES + [ { id: "4", path: "secret.md", content: "", vault_id: "vault-4" } ],
                 vaults: VAULTS + [ { id: "vault-4", tenant_id: "tenant-1", name: "Board", members: [ "bia@markupp.dev" ] } ])
    sign_in_as

    get note_path("4")

    assert_select "button[popovertarget=vault_switcher]", text: "Research"
    assert_select "nav [role=treeitem]", text: "└── plan"
  end

  # O ![[nota]] embutido busca a nota em json: embutir não é abrir, e o cofre ativo fica.
  test "fetching a note of another vault as json keeps the active vault" do
    get note_path("2", format: :json)

    get root_path
    assert_select "button[popovertarget=vault_switcher]", text: "Research"
  end

  test "a new note goes to the active vault" do
    patch current_vault_path, params: { vault_id: "vault-2" }

    post notes_path, params: { note: { path: "/idea", content: "x" }, origin_path: "" }

    assert_redirected_to note_path("created-1")
    assert_includes @api.received_credentials.pluck(:call), "POST /v1/vaults/vault-2/notes"
  end

  test "the search looks only in the active vault" do
    get search_path(q: "ship")

    assert_equal %w[plan], css_select("[role=option] .search-hit-title").map(&:text)
  end

  test "the flat explorer views list the notes of the active vault" do
    patch current_vault_path, params: { vault_id: "vault-2" }

    get explorer_path(mode: "central")

    assert_select "turbo-frame#explorer_list a", text: /draft/
    assert_select "turbo-frame#explorer_list a", text: /plan/, count: 0
  end

  test "the full graph draws the active vault" do
    patch current_vault_path, params: { vault_id: "vault-2" }

    get graph_path

    assert_select "[data-graph-view-url-value=?]", graph_path(format: :json, vault: "vault-2")
  end

  test "without a vault the sidebar shows an empty state with New vault in place of the tree" do
    use_fake_api(notes: NOTES, vaults: [])
    sign_in_as

    get root_path

    assert_response :success
    assert_select "nav [role=tree]", count: 0
    assert_select "nav", text: /No vault yet/
    assert_select "nav button[commandfor=new_vault][command=show-modal]", text: /New vault/
    assert_select "main textarea", count: 0
  end

  test "a viewer without a vault is told to ask for one, with no New vault" do
    use_fake_api(notes: NOTES, vaults: [], tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "viewer" } } ])
    sign_in_as

    get root_path

    assert_select "nav", text: /Ask a member of a vault to add you/
    assert_select "nav button[commandfor=new_vault]", count: 0
  end
end
