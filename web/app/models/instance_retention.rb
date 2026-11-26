# Prazo de guarda do histórico e do IP, como GET /v1/instance/retention devolve (ADR-0061). host_locked é o prazo fixado
# por quem hospeda, que o painel só mostra.
InstanceRetention = Data.define(:days, :host_locked) do
  self::PATH = MarkuppApi.path("v1", "instance", "retention")

  def self.fetch(api) = from_api(api.get(self::PATH))

  def self.update(api, days) = from_api(api.put(self::PATH, { days: }))

  def self.from_api(attributes) = new(days: attributes.fetch("days"), host_locked: attributes.fetch("host_locked"))
end
