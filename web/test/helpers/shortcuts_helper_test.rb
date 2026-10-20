require "test_helper"

class ShortcutsHelperTest < ActionView::TestCase
  test "shortcut_label spaces the keys out and shortens Delete" do
    assert_equal "Alt Shift C", shortcut_label("Alt+Shift+C")
    assert_equal "Del", shortcut_label("Delete")
    assert_equal "Ctrl ,", shortcut_label("Ctrl+,")
  end

  test "help_keys_width is the length of the longest keys line of the help" do
    assert_equal "Shift+F10 / Menu".length, help_keys_width
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
end
