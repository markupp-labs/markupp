require "test_helper"

# Publicação do cofre pelo painel (ADR-0048): ler, publicar com slug e título e despublicar, como owner ou admin.
class VaultPublicationTest < ActiveSupport::TestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  PUBLICATIONS = [ { vault_id: "vault-2", slug: "handbook", title: "Handbook" } ].freeze

  test "find answers nil for a vault that is not published" do
    assert_nil VaultPublication.find(signed_in_api, "vault-1")
  end

  test "find builds the publication of a published vault" do
    publication = VaultPublication.find(signed_in_api, "vault-2")

    assert_equal [ "vault-2", "handbook", "Handbook" ], [ publication.vault_id, publication.slug, publication.title ]
    assert_equal Time.utc(2026, 10, 20, 12), publication.published_at
  end

  test "publish answers the new publication with the slug and the title" do
    api = signed_in_api

    publication = VaultPublication.publish(api, "vault-1", slug: "docs", title: "Docs")

    assert_equal [ "vault-1", "docs", "Docs" ], [ publication.vault_id, publication.slug, publication.title ]
    assert_equal "docs", VaultPublication.find(api, "vault-1").slug
  end

  test "publish raises SlugTaken for a slug of another vault" do
    assert_raises(MarkuppApi::SlugTaken) { VaultPublication.publish(signed_in_api, "vault-1", slug: "handbook", title: "") }
  end

  test "publish raises InvalidRequest for a slug out of the format" do
    assert_raises(MarkuppApi::InvalidRequest) { VaultPublication.publish(signed_in_api, "vault-1", slug: "No Spaces", title: "") }
  end

  test "publish raises Forbidden for a member" do
    assert_raises(MarkuppApi::Forbidden) { VaultPublication.publish(signed_in_api(role: "member"), "vault-1", slug: "docs", title: "") }
  end

  test "unpublish takes the vault out of the public reading" do
    api = signed_in_api

    VaultPublication.unpublish(api, "vault-2")

    assert_nil VaultPublication.find(api, "vault-2")
  end

  test "unpublish raises NotPublished, a kind of NotFound, for a vault that is not published" do
    assert_raises(MarkuppApi::NotFound) { VaultPublication.unpublish(signed_in_api, "vault-1") }
  end

  private
    def signed_in_api(role: "owner")
      tenants = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => role } } ]
      fake_api = FakeMarkuppApi.new(tenants:, publications: PUBLICATIONS)
      pair = fake_api.post("/auth/login", email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
      fake_api.as(access_token: pair.fetch("access_token"), tenant_id: "tenant-1")
    end
end
