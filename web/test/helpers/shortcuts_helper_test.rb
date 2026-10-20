require "test_helper"

class ShortcutsHelperTest < ActionView::TestCase
  test "shortcut_label spaces the keys out and shortens Delete" do
    assert_equal "Alt Shift C", shortcut_label("Alt+Shift+C")
    assert_equal "Del", shortcut_label("Delete")
    assert_equal "Ctrl ,", shortcut_label("Ctrl+,")
  end

  test "help_keys_width is the length of the longest keys line of the help" do
    assert_equal "Ctrl+Shift+P / F1 / :".length, help_keys_width
  end

  test "the help has the vim group only with vim keys on" do
    @vim_keys = "off"
    assert_not_includes help_shortcut_groups.keys, :vim

    @vim_keys = "on"
    assert_equal [ ":w", ":wq", ":q", ":q!", ":e path", ":edit", ":e", ":read", ":split", "Esc Esc Tab", "Esc Esc Shift+Tab" ],
                 help_shortcut_groups[:vim]&.flat_map(&:keys)
  end

  test "shortcut_keys reads the keys of a shortcut by its name, one name per shortcut" do
    assert_equal [ "j", "↓" ], shortcut_keys(:next_row)
    assert_equal "Alt+Shift+C", shortcut_key(:copy_path)
    assert_equal SHORTCUT_GROUPS.values.flatten.size, SHORTCUTS_BY_NAME.size
    assert_raises(KeyError) { shortcut_keys(:missing) }
  end

  # O note_tree_controller.js trata as teclas das linhas sem passar pelo servidor: o espelho tem as mesmas teclas.
  test "the row shortcuts of the tree controller mirror the registry" do
    source = Rails.root.join("app/javascript/controllers/note_tree_controller.js").read
    mirror = source[/const ROW_SHORTCUTS = \{(.*?)\}/, 1].scan(/"?([\w+]+)"?: "(\w+)"/).to_h do |key, action|
      [ action.underscore.to_sym, key.sub("alt+shift+Key", "Alt+Shift+") ]
    end

    assert_equal mirror.keys.index_with { shortcut_key(it) }, mirror
  end

  private
    # O modo Vim vem do cookie (SettingsHelper); aqui, do teste.
    def chosen_vim_keys = @vim_keys || "off"
end
