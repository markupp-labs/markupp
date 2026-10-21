module TenantsHelper
  # Linha de estatísticas do card ("12 notes, last edit 2h ago"); nil quando o servidor não manda as estatísticas.
  def tenant_stats(tenant)
    return if tenant.notes_count.nil?
    return t("tenants.card.no_notes") if tenant.notes_count.zero?

    count = t("tenants.card.notes", count: tenant.notes_count)
    return count unless tenant.last_note_at

    "#{count}, #{t("tenants.card.last_edit", time: time_ago_in_words(tenant.last_note_at, scope: :datetime_short))}"
  end
end
