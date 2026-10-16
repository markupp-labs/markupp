# Mesmos verbos do MarkuppApi, com a credencial da sessão: renova o token de acesso vencido antes de chamar
# e, se a API ainda responder 401, renova uma vez e repete a chamada uma vez.
# A API rotaciona o refresh a cada uso e, se um refresh usado voltar, derruba todas as sessões da conta. Por isso
# requisições simultâneas com o mesmo refresh renovam uma vez só: a trava é pelo digest do refresh e o par novo
# fica em renewed_pairs por RENEWED_PAIR_TTL, também pelo digest, para a requisição que chega depois reaproveitar.
class RefreshingMarkuppApi
  RENEWED_PAIR_TTL = 30.seconds
  # ponytail: travas do processo, valem para as threads do Puma; com mais de um processo ou réplica,
  # só o memo em renewed_pairs protege, e ele precisa de cache compartilhado entre eles.
  RENEWAL_LOCKS = Array.new(64) { Mutex.new }

  def initialize(credential, renewed_pairs: Rails.cache)
    @credential = credential
    @renewed_pairs = renewed_pairs
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
      @credential.sign_in(renewed_pair(@credential.refresh_token))
    rescue MarkuppApi::RefreshRejected, MarkuppApi::Unauthorized => error
      raise MarkuppApi::SessionExpired, "#{call} needed a new access token, but POST /auth/refresh refused the refresh token " \
                                        "of the session, expected a valid one: #{error.message}"
    end

    # Só o digest vira chave: o refresh cru não vai para o cache nem para o log do cache.
    def renewed_pair(refresh_token)
      digest = Digest::SHA256.hexdigest(refresh_token.to_s)
      RENEWAL_LOCKS[digest.hex % RENEWAL_LOCKS.size].synchronize do
        @renewed_pairs.fetch("panel/renewed_pair/#{digest}", expires_in: RENEWED_PAIR_TTL) do
          @credential.public_api.post("/auth/refresh", refresh_token:)
        end
      end
    end
end
