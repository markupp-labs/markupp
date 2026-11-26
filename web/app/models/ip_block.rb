# IP ou faixa que a instância recusa em toda rota, como GET /v1/instance/ip-blocks devolve (ADR-0061).
IpBlock = Data.define(:id, :cidr, :note, :created_at) do
  extend TimestampedFromApi

  def self.all(api) = api.get(path).map { from_api(it) }

  def self.create(api, cidr:, note:) = from_api(api.post(path, { cidr:, note: }))

  def self.destroy(api, id) = api.delete(path(id))

  def self.path(*id) = MarkuppApi.path("v1", "instance", "ip-blocks", *id)

  def self.from_api(attributes) = timestamped_from(attributes, :created_at)
end
