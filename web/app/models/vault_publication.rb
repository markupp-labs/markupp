# Publicação do cofre para a leitura sem conta (ADR-0048), como GET /v1/vaults/{id}/publication devolve. Só owner e
# admin publicam e despublicam; a regra do slug e o papel ficam na API.
VaultPublication = Data.define(:vault_id, :slug, :title, :published_by, :published_at) do
  # A publicação do cofre, ou nil quando ele não está publicado.
  def self.find(api, vault_id)
    from_api(api.get(api_path(vault_id)))
  rescue MarkuppApi::NotPublished
    nil
  end

  # Publica com o slug e o título, ou troca os dois; título vazio fica com o nome do cofre.
  def self.publish(api, vault_id, slug:, title:) = from_api(api.put(api_path(vault_id), slug:, title:))

  def self.unpublish(api, vault_id) = api.delete(api_path(vault_id))

  def self.api_path(vault_id) = MarkuppApi.path("v1", "vaults", vault_id, "publication")

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, published_at: Time.zone.parse(fields.fetch(:published_at)))
  end

  private_class_method :api_path
end
