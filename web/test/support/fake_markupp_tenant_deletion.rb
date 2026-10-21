# DELETE /tenants/{id} da FakeMarkuppApi: como o servidor, o tenant vem do caminho, só o owner apaga e quem não é membro
# recebe tenant_not_found, sem saber se o tenant existe. Apagado, some com os vínculos e os convites dele.
class FakeMarkuppTenantDeletion
  TENANT_PATH = %r{\A/tenants/(?<id>[^/]+)\z}

  def initialize(tenants)
    @tenants = tenants
  end

  def delete(path, account:, **)
    id = TENANT_PATH.match(path)&.[](:id) or raise ArgumentError, "fake API has no route for DELETE #{path}, expected /tenants/{id}"
    role = @tenants.tenant(id)&.dig("members", account["email"])
    raise MarkuppApi::TenantNotFound, "fake API refuses DELETE #{path}: tenant_not_found, #{account["email"]} is not in #{id.inspect}" unless role
    raise MarkuppApi::Forbidden, "fake API refuses DELETE #{path} for role #{role.inspect}: forbidden, expected owner" unless role == "owner"

    @tenants.delete_tenant!(id)
    nil
  end
end
