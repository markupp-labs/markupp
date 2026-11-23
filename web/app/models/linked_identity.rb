# Identidade externa vinculada à conta logada, como GET /v1/me/identities devolve. A senha não aparece aqui.
LinkedIdentity = Data.define(:id, :provider) do
  def self.mine(api) = api.get(MarkuppApi.path("v1", "me", "identities")).map { new(id: it.fetch("id"), provider: it.fetch("provider")) }

  # Desvincula; a última forma de entrar levanta MarkuppApi::LastIdentity.
  def self.unlink(api, id) = api.delete(MarkuppApi.path("v1", "me", "identities", id))
end
