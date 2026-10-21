require "test_helper"

# Linguagens dos blocos cercados no editor: mapeadas no importmap, mas fora do modulepreload, para só baixarem
# quando a nota tiver um bloco daquela linguagem.
class EditorLanguagesTest < ActionDispatch::IntegrationTest
  LAZY_MODES = %w[diff go python ruby shell sql yaml javascript].map { |mode| "@codemirror/legacy-modes/mode/#{mode}" }

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as
    get note_path("1")
  end

  test "the language modes are mapped in the importmap" do
    imports = JSON.parse(css_select("script[type=importmap]").sole.text)["imports"]

    assert_empty LAZY_MODES - imports.keys, "expected every language mode in the importmap, got #{imports.keys.grep(/legacy/)}"
  end

  test "the language modes are left out of the initial download" do
    preloads = css_select("link[rel=modulepreload]").map { |link| link["href"] }

    assert_empty preloads.grep(/legacy-modes/), "expected no language mode preloaded, got #{preloads.grep(/legacy-modes/)}"
  end
end
