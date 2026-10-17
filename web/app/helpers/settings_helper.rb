module SettingsHelper
  # Opções do select de papel: os papéis que o tenant ativo concede, com o nome traduzido.
  def role_options(tenant) = tenant.assignable_roles.map { [ t("tenants.roles.#{it}"), it ] }

  # Tema gravado no cookie, ou nil quando o painel segue o sistema.
  def chosen_theme = cookies[:theme].presence_in(ThemesController::THEMES)

  # Item da navegação das configurações; a seção aberta leva o mesmo realce da nota atual.
  def settings_nav_link(section, path)
    link_to t("settings.nav.#{section}"), path, aria: { current: ("page" if section == settings_section) },
            class: "tree-row block whitespace-nowrap aria-[current=page]:bg-accent-soft aria-[current=page]:text-accent"
  end
end
