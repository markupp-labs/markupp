require "test_helper"

class CompactAgeHelperTest < ActionView::TestCase
  NOW = Time.utc(2026, 10, 21, 12)
  AGES = [ 30.seconds, 5.minutes, 2.hours, 3.days, 15.days, 40.days, 800.days ].freeze

  test "the age takes the largest step that fits, in English" do
    assert_equal %w[now 5m 2h 3d 2w 1mo 2y], AGES.map { compact_age(NOW - it, now: NOW) }
  end

  test "the units come from the locale" do
    I18n.with_locale(:"pt-BR") do
      assert_equal %w[agora 5min 2h 3d 2sem 1mês 2a], AGES.map { compact_age(NOW - it, now: NOW) }
    end
  end

  test "no time gives no age" do
    assert_equal "", compact_age(nil)
  end

  test "below a minute there is no step" do
    assert_nil age_step(NOW - 30.seconds, now: NOW)
    assert_equal [ 5, "m" ], age_step(NOW - 5.minutes, now: NOW)
  end
end
