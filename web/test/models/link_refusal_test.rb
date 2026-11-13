require "test_helper"

class LinkRefusalTest < ActiveSupport::TestCase
  ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
  BIA = "bia@markupp.dev"
  PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
  ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: BIA, password: PASSWORD } ].freeze
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { ANA => "member", BIA => "viewer" } } ].freeze
  NOTES = [ { id: "plan", path: "work/plan.md", content: "" }, { id: "roadmap", path: "roadmap.md", content: "" },
            { id: "ideas", path: "ideas.md", content: "" }, { id: "elsewhere", path: "elsewhere.md", content: "", vault_id: "vault-2" } ].freeze
  REFUSALS = [ { id: "r1", note_ids: %w[roadmap plan] } ].freeze

  setup do
    @fake_api = FakeMarkuppApi.new(accounts: ACCOUNTS, tenants: TENANTS, notes: NOTES, link_refusals: REFUSALS)
  end

  test "all builds the refusals of the vault with both notes and the date" do
    refusal = LinkRefusal.all(api_as(ANA), "vault-1").sole

    assert_equal "r1", refusal.id
    assert_equal [ [ "plan", "plan", "work" ], [ "roadmap", "roadmap", "" ] ], refusal.notes.map { [ it.id, it.title, it.folder ] }
    assert_equal Time.utc(2026, 11, 1, 12), refusal.created_at
  end

  test "refuse adds the pair at the top, and refusing it again in the other order keeps one" do
    api = api_as(ANA)

    LinkRefusal.refuse(api, "vault-1", %w[ideas plan])
    LinkRefusal.refuse(api, "vault-1", %w[plan ideas])

    assert_equal [ %w[ideas plan], %w[plan roadmap] ], LinkRefusal.all(api, "vault-1").map { it.notes.map(&:id) }
  end

  test "undo takes the refusal away" do
    api = api_as(ANA)

    assert_nil LinkRefusal.undo(api, "vault-1", "r1")
    assert_empty LinkRefusal.all(api, "vault-1")
  end

  test "refuse raises SameNote for one note twice and NoteOutsideVault for a note of another vault" do
    api = api_as(ANA)

    assert_raises(MarkuppApi::SameNote) { LinkRefusal.refuse(api, "vault-1", %w[plan plan]) }
    assert_raises(MarkuppApi::NoteOutsideVault) { LinkRefusal.refuse(api, "vault-1", %w[plan elsewhere]) }
  end

  test "undo raises LinkRefusalNotFound for a refusal already undone" do
    assert_raises(MarkuppApi::LinkRefusalNotFound) { LinkRefusal.undo(api_as(ANA), "vault-1", "r9") }
  end

  test "a viewer reads the refusals but neither refuses nor undoes" do
    api = api_as(BIA)

    assert_equal [ "r1" ], LinkRefusal.all(api, "vault-1").map(&:id)
    assert_raises(MarkuppApi::ReadOnly) { LinkRefusal.refuse(api, "vault-1", %w[ideas plan]) }
    assert_raises(MarkuppApi::ReadOnly) { LinkRefusal.undo(api, "vault-1", "r1") }
  end

  private
    def api_as(email)
      pair = @fake_api.post("/auth/login", email:, password: PASSWORD)
      @fake_api.as(access_token: pair.fetch("access_token"), tenant_id: "tenant-1")
    end
end
