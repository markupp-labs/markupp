# Sessão de login da conta, como GET /v1/me/sessions devolve (ADR-0061): o login que as renovações continuam, com o IP e o
# user agent do último uso. current é a sessão deste painel, que sai pelo Log out e não pelo encerrar.
AccountSession = Data.define(:id, :ip, :user_agent, :created_at, :last_used_at, :current) do
  extend V1Listing

  def self.mine(api) = list_from(api, "me", "sessions")

  def self.end(api, id) = api.delete(MarkuppApi.path("v1", "me", "sessions", id))

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, created_at: Time.zone.parse(fields.fetch(:created_at)), last_used_at: Time.zone.parse(fields.fetch(:last_used_at)))
  end
end
