# Idade curta como a do ls e do git log --relative ("5m", "2h", "3d"), na coluna do Explorer e nos cards de tenant. As
# unidades vêm do idioma, em compact_age.<unidade>.
module CompactAgeHelper
  # O maior passo que cabe ganha.
  AGE_STEPS = { "y" => 1.year, "mo" => 30.days, "w" => 1.week, "d" => 1.day, "h" => 1.hour, "m" => 1.minute }.freeze

  def compact_age(time, now: Time.current)
    return "" if time.nil?

    count, unit = age_step(time, now:)
    unit ? t("compact_age.#{unit}", count:) : t("compact_age.now")
  end

  # Quantos passos inteiros e de qual unidade; nil abaixo do menor passo.
  def age_step(time, now: Time.current)
    seconds = now - time
    unit, span = AGE_STEPS.find { |_unit, step| seconds >= step }
    [ (seconds / span).floor, unit ] if unit
  end
end
