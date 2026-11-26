# Nome curto do cliente de uma sessão ou de um app autorizado, a partir do user agent: "Firefox no Linux" para o
# navegador, o nome do produto ("Claude-User", "curl") para o resto. A ordem importa: o Edge e o Chrome também dizem
# Safari, e o Android também diz Linux.
module ClientLabelHelper
  BROWSERS = { "Edge" => /Edg\//, "Opera" => /OPR\//, "Firefox" => /Firefox\//, "Chrome" => /Chrome\//, "Safari" => /Safari\// }.freeze
  SYSTEMS = { "Android" => /Android/, "iOS" => /iPhone|iPad/, "Windows" => /Windows/, "macOS" => /Mac OS X/, "Linux" => /Linux/ }.freeze

  def client_label(user_agent)
    return t("client_label.unknown") if user_agent.blank?

    browser = BROWSERS.find { |_name, pattern| pattern.match?(user_agent) }&.first
    return user_agent[%r{\A[^/\s]+}] unless browser

    system = SYSTEMS.find { |_name, pattern| pattern.match?(user_agent) }&.first
    system ? t("client_label.on_system", browser:, system:) : browser
  end
end
