require "test_helper"

class VaultMemberTest < ActiveSupport::TestCase
  ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
  BIA = "bia@markupp.dev"
  CAIO = "caio@markupp.dev"
  PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
  ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: BIA, password: PASSWORD } ].freeze
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { ANA => "owner", BIA => "viewer", CAIO => "member" } } ].freeze
  VAULTS = [ { id: "vault-1", tenant_id: "tenant-1", name: "Research", members: [ ANA, BIA ] } ].freeze
  JOINED_AT = Time.utc(2026, 9, 1, 12)

  setup do
    @fake_api = FakeMarkuppApi.new(accounts: ACCOUNTS, tenants: TENANTS, vaults: VAULTS)
  end

  test "all builds the members of the vault with their tenant role and join date" do
    assert_equal [ VaultMember.new(id: "account-1", email: ANA, role: "owner", joined_at: JOINED_AT),
                   VaultMember.new(id: "account-2", email: BIA, role: "viewer", joined_at: JOINED_AT) ],
                 VaultMember.all(api_as(ANA), "vault-1")
  end

  test "add makes a colleague of the tenant a member of the vault" do
    api = api_as(ANA)

    VaultMember.add(api, "vault-1", "account-caio")

    assert_equal [ ANA, BIA, CAIO ], VaultMember.all(api, "vault-1").map(&:email)
  end

  test "add raises AlreadyVaultMember for a member and NotTenantMember for someone outside the tenant" do
    api = api_as(ANA)

    assert_raises(MarkuppApi::AlreadyVaultMember) { VaultMember.add(api, "vault-1", "account-2") }
    assert_raises(MarkuppApi::NotTenantMember) { VaultMember.add(api, "vault-1", "account-9") }
  end

  test "remove drops the member from the vault" do
    api = api_as(ANA)

    assert_nil VaultMember.remove(api, "vault-1", "account-2")
    assert_equal [ ANA ], VaultMember.all(api, "vault-1").map(&:email)
  end

  test "a viewer can leave the vault but not remove someone else" do
    api = api_as(BIA)

    assert_raises(MarkuppApi::ReadOnly) { VaultMember.remove(api, "vault-1", "account-1") }
    VaultMember.remove(api, "vault-1", "account-2")
    assert_equal [ ANA ], VaultMember.all(api_as(ANA), "vault-1").map(&:email)
  end

  test "remove raises VaultMemberNotFound for someone outside the vault" do
    assert_raises(MarkuppApi::VaultMemberNotFound) { VaultMember.remove(api_as(ANA), "vault-1", "account-caio") }
  end

  test "candidates are the colleagues of the tenant that are not members yet" do
    api = api_as(ANA)

    assert_equal [ VaultMember::Candidate.new(id: "account-caio", email: CAIO) ],
                 VaultMember.candidates(api, VaultMember.all(api, "vault-1"))
  end

  private
    def api_as(email)
      pair = @fake_api.post("/auth/login", email:, password: PASSWORD)
      @fake_api.as(access_token: pair.fetch("access_token"), tenant_id: "tenant-1")
    end
end
