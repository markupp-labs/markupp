module SettingsHelper
  # Opções do select de papel: os papéis que o tenant ativo concede, com o nome traduzido.
  def role_options(tenant) = tenant.assignable_roles.map { [ t("tenants.roles.#{it}"), it ] }

  # Tema gravado no cookie, ou nil quando o painel segue o sistema.
  def chosen_theme = cookies[:theme].presence_in(ThemesController::THEMES)

  # Estilo do editor gravado no cookie; sem escolha, o plain do VS Code.
  def chosen_editor_style = cookies[:editor_style].presence_in(EditorStylesController::STYLES) || "plain"

  # Modo Vim do editor gravado no cookie; sem escolha, desligado.
  def chosen_vim_keys = cookies[:vim_keys].presence_in(VimKeysController::CHOICES) || "off"

  # Item da navegação das configurações; a seção aberta leva o mesmo realce da nota atual.
  def settings_nav_link(section, path)
    link_to t("settings.nav.#{section}"), path, aria: { current: ("page" if section == settings_section) },
            class: "tree-row block whitespace-nowrap aria-[current=page]:bg-accent-soft aria-[current=page]:text-ink"
  end
end
