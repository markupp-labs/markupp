require "test_helper"

class VaultTest < ActiveSupport::TestCase
  ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
  BIA = "bia@markupp.dev"
  PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
  ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: BIA, password: PASSWORD } ].freeze
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { ANA => "owner", BIA => "viewer" } } ].freeze
  VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ ANA, BIA ] },
             { id: "vault-2", tenant_id: "tenant-1", name: "Drafts", members: [ ANA ] } ].freeze
  CREATED_AT = Time.utc(2026, 9, 1, 12)

  setup do
    @fake_api = FakeMarkuppApi.new(accounts: ACCOUNTS, tenants: TENANTS, vaults: VAULTS)
  end

  test "mine builds the vaults the account is a member of" do
    assert_equal [ Vault.new(id: "vault-1", name: "Research", created_at: CREATED_AT), Vault.new(id: "vault-2", name: "Drafts", created_at: CREATED_AT) ],
                 Vault.mine(api_as(ANA))
    assert_equal [ "vault-1" ], Vault.mine(api_as(BIA)).map(&:id)
  end

  test "pick answers the vault of the session while it is still in the list" do
    assert_equal "vault-2", Vault.pick(Vault.mine(api_as(ANA)), "vault-2").id
  end

  test "pick answers the first vault when the session has none or a stale one" do
    vaults = Vault.mine(api_as(ANA))

    assert_equal "vault-1", Vault.pick(vaults, nil).id
    assert_equal "vault-1", Vault.pick(vaults, "vault-9").id
  end

  test "pick answers nil without vaults" do
    assert_nil Vault.pick([], "vault-1")
  end

  test "create answers the new vault, which the account is a member of" do
    api = api_as(ANA)
    vault = Vault.create(api, "Field notes")

    assert_equal "Field notes", vault.name
    assert_includes Vault.mine(api), vault
  end

  test "create strips the spaces at the ends of the name" do
    assert_equal "Field notes", Vault.create(api_as(ANA), "  Field notes ").name
  end

  test "create raises InvalidName for a name the API refuses" do
    assert_raises(MarkuppApi::InvalidName) { Vault.create(api_as(ANA), " ") }
  end

  test "create by a viewer raises ReadOnly" do
    assert_raises(MarkuppApi::ReadOnly) { Vault.create(api_as(BIA), "Field notes") }
  end

  test "rename answers the vault with the new name" do
    api = api_as(ANA)

    assert_equal "Papers", Vault.rename(api, "vault-1", " Papers ").name
    assert_equal "Papers", Vault.mine(api).first.name
  end

  test "rename of a vault the account is not a member of raises VaultUnavailable" do
    assert_raises(MarkuppApi::VaultUnavailable) { Vault.rename(api_as(BIA), "vault-2", "Papers") }
  end

  test "delete removes the vault" do
    api = api_as(ANA)

    assert_nil Vault.delete(api, "vault-2")
    assert_equal [ "vault-1" ], Vault.mine(api).map(&:id)
  end

  private
    def api_as(email)
      pair = @fake_api.post("/auth/login", email:, password: PASSWORD)
      @fake_api.as(access_token: pair.fetch("access_token"), tenant_id: "tenant-1")
    end
end
