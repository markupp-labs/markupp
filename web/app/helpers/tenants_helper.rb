module TenantsHelper
  # Linha de estatísticas do card ("12 notes, last edit 2h ago"); nil quando o servidor não manda as estatísticas.
  def tenant_stats(tenant)
    return if tenant.notes_count.nil?
    return t("tenants.card.no_notes") if tenant.notes_count.zero?

    count = t("tenants.card.notes", count: tenant.notes_count)
    return count unless tenant.last_note_at

    "#{count}, #{last_edit(tenant.last_note_at)}"
  end

  # Abaixo de um minuto a idade curta seria "now", que não cabe em "last edit %{time} ago".
  def last_edit(time)
    return t("tenants.card.last_edit_now") unless age_step(time)

    t("tenants.card.last_edit", time: compact_age(time))
  end
end
