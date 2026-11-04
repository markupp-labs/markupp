# Contador do sino da barra de atividades (notifications/_badge).
module NotificationsHelper
  BADGE_LIMIT = 99

  # O número no sino, que para em 99+ para caber no quadrado de 48px.
  def notification_badge_count(unread) = unread > BADGE_LIMIT ? "#{BADGE_LIMIT}+" : unread.to_s

  # O nome do botão para o leitor de tela, com a contagem inteira; sem pendência é só o sino.
  def notification_label(unread) = t("notifications.badge.label", count: unread)
end
