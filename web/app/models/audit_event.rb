# Evento do histórico de acessos e mudanças sensíveis, como GET /v1/instance/audit-events devolve (ADR-0061). actor_email
# vem vazio quando não há conta, como na falha de login de email sem conta; aí quem tentou está em target.
AuditEvent = Data.define(:id, :actor_id, :actor_email, :action, :target, :result, :ip, :user_agent, :occurred_at) do
  extend TimestampedFromApi

  self::PAGE_SIZE = 50
  self::RESULTS = %w[success failure blocked].freeze

  # filters: account, event (a ação), result, ip e as datas since e until (AAAA-MM-DD, until inclusive).
  def self.page(api, filters, offset: 0)
    query = { account: filters[:account], action: filters[:event], ip: filters[:ip] }.transform_values(&:presence)
    query.merge!(result: filters[:result].presence_in(self::RESULTS), since: day_start(filters[:since]), until: day_start(filters[:until], 1))
    api.get(MarkuppApi.path("v1", "instance", "audit-events", **query, limit: self::PAGE_SIZE, offset:)).map { from_api(it) }
  end

  # Início do dia, em UTC; days avança, para o until da página valer o dia inteiro. Data inválida fica de fora.
  def self.day_start(date, days = 0)
    return if date.blank?

    (Date.iso8601(date) + days).to_time(:utc).iso8601
  rescue Date::Error
    nil
  end

  def self.from_api(attributes) = timestamped_from(attributes, :occurred_at)

  # Quem fez: o email da conta ou, sem conta, o alvo.
  def who = actor_email.presence || target
end
