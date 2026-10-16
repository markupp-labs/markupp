# Mesmos verbos do MarkuppApi, com a credencial da sessão: renova o token de acesso vencido antes de chamar
# e, se a API ainda responder 401, renova uma vez e repete a chamada uma vez.
# ponytail: duas requisições com o token vencido renovam em paralelo e a API, ao ver o refresh reusado,
# derruba a sessão; trava entre requisições só com a sessão guardada no servidor.
class RefreshingMarkuppApi
  def initialize(credential)
    @credential = credential
  end

  def get(path) = with_fresh_token("GET #{path}") { it.get(path) }

  def put(path, body) = with_fresh_token("PUT #{path}") { it.put(path, body) }

  def patch(path, body) = with_fresh_token("PATCH #{path}") { it.patch(path, body) }

  def post(path, body) = with_fresh_token("POST #{path}") { it.post(path, body) }

  def delete(path) = with_fresh_token("DELETE #{path}") { it.delete(path) }

  private
    def with_fresh_token(call)
      renew(call) if @credential.access_expired?
      yield @credential.bearer_api
    rescue MarkuppApi::Unauthorized
      renew(call)
      yield @credential.bearer_api
    end

    def renew(call)
      @credential.sign_in(@credential.public_api.post("/auth/refresh", refresh_token: @credential.refresh_token))
    rescue MarkuppApi::RefreshRejected, MarkuppApi::Unauthorized => error
      raise MarkuppApi::SessionExpired, "#{call} needed a new access token, but POST /auth/refresh refused the refresh token " \
                                        "of the session, expected a valid one: #{error.message}"
    end
end
