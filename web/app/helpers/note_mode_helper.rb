# Modo da nota (edição, leitura ou lado a lado), como no Obsidian e no VS Code. O último escolhido mora no
# cookie note_mode, gravado pelo note_mode_controller.js e lido aqui para o servidor já desenhar o modo certo.
module NoteModeHelper
  # Ícone e tecla de cada modo. Ctrl+E alterna edição e leitura (Obsidian); Ctrl+\ alterna o lado a lado (split do VS Code).
  NOTE_MODE_BUTTONS = { "edit" => [ "pencil-simple", "E" ], "read" => [ "book-open", "E" ], "split" => [ "columns", "\\" ] }.freeze
  NOTE_MODES = NOTE_MODE_BUTTONS.keys.freeze

  # Nota nova sempre abre em edição; mode desconhecido volta para edição.
  def note_mode(raw, persisted:)
    return "edit" unless persisted

    raw.presence_in(NOTE_MODES) || "edit"
  end

  def current_note_mode(note) = note_mode(cookies[:note_mode], persisted: note.persisted?)

  # Botão só de ícone do seletor; aria-pressed marca o modo atual e o title mostra o atalho.
  def note_mode_button(mode, current:)
    icon_name, key = NOTE_MODE_BUTTONS.fetch(mode)
    label = t("notes.form.modes.#{mode}")
    tag.button(type: "button", title: "#{label} (Ctrl+#{key})", class: "btn-icon note-mode-#{mode} aria-pressed:bg-accent-soft aria-pressed:text-ink",
               aria: { label:, pressed: mode == current, keyshortcuts: "Control+#{key} Meta+#{key}" },
               data: { action: "note-mode#choose", note_mode_mode_param: mode, note_mode_target: "button" }) { icon(icon_name) }
  end
end
