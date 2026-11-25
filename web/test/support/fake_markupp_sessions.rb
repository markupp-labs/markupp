# Sessões de login da FakeMarkuppApi: GET /v1/me/sessions e DELETE /v1/me/sessions/{id}, como a API (ADR-0061). A
# sessão inexistente ou já encerrada é o mesmo 404.
class FakeMarkuppSessions
  ROUTE = %r{\A/v1/me/sessions(?:/(?<id>[^/]+))?\z}
  FIELDS = %w[id ip user_agent created_at last_used_at current].freeze

  # sessions: [{ id:, current:, ip:, user_agent: }], na ordem da API; o resto ganha um padrão.
  def initialize(sessions)
    @sessions = sessions.map { default_session(it[:id]).merge(it.stringify_keys) }
  end

  def ids = @sessions.map { it["id"] }

  def get(path, account:, **)
    refuse_anonymous(account, path)
    @sessions.map { it.slice(*FIELDS) }
  end

  def delete(path, account:, **)
    refuse_anonymous(account, path)
    id = ROUTE.match(path)[:id]
    @sessions.reject! { it["id"] == id } or
      raise MarkuppApi::SessionNotFound, "fake API has no session #{id.inspect}: session_not_found, expected one of #{ids}"
    nil
  end

  private
    def default_session(id)
      { "id" => id, "current" => false, "ip" => "203.0.113.1", "user_agent" => "", "created_at" => "2026-11-20T12:00:00Z",
        "last_used_at" => 10.minutes.ago.utc.iso8601 }
    end

    def refuse_anonymous(account, path)
      raise MarkuppApi::Unauthorized, "fake API refuses #{path} without a login token" unless account
    end
end
