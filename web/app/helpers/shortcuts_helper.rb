# Atalhos de teclado do painel.
module ShortcutsHelper
  KEY_LABELS = { "+" => " ", "Delete" => "Del" }.freeze

  # Rótulo curto da tecla para o <kbd>, como no menu do VS Code: Alt+Shift+C vira "Alt Shift C", Delete vira "Del".
  def shortcut_label(keys) = keys.gsub(/\+|Delete/, KEY_LABELS)
end
