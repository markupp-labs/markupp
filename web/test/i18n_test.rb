require "test_helper"
require "i18n/tasks"

# Toda chave usada existe em todos os idiomas do painel, nenhuma sobra e os %{} batem entre eles.
# A ordem das chaves não é conferida: o normalize do i18n-tasks apaga os comentários dos arquivos.
class I18nTest < ActiveSupport::TestCase
  setup { @i18n_tasks = I18n::Tasks::BaseTask.new }

  test "every used key is translated in every locale" do
    missing = @i18n_tasks.missing_keys

    assert_empty missing, "expected no missing keys, got #{missing.leaves.map(&:full_key).first(20)}; run i18n-tasks missing"
  end

  test "every translated key is used" do
    unused = @i18n_tasks.unused_keys

    assert_empty unused, "expected no unused keys, got #{unused.leaves.map(&:full_key).first(20)}; run i18n-tasks unused"
  end

  test "interpolations match across locales" do
    inconsistent = @i18n_tasks.inconsistent_interpolations

    assert_empty inconsistent, "expected the same %{} in every locale, got #{inconsistent.leaves.map(&:full_key)}"
  end
end
