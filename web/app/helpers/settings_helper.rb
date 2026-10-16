module SettingsHelper
  # Opções do select de papel: os papéis que o tenant ativo concede, com o nome traduzido.
  def role_options(tenant) = tenant.assignable_roles.map { [ t("tenants.roles.#{it}"), it ] }
end
