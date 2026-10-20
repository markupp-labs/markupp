# Mesmos verbos do MarkuppApi, com a credencial da sessão: renova o token de acesso vencido antes de chamar
# e, se a API ainda responder 401, renova uma vez e repete a chamada uma vez.
# A API rotaciona o refresh a cada uso e, se um refresh usado voltar, derruba todas as sessões da conta. Por isso
# requisições simultâneas com o mesmo refresh renovam uma vez só: a trava é pelo digest do refresh e o par novo
# fica em renewed_pairs por RENEWED_PAIR_TTL, também pelo digest, para a requisição que chega depois reaproveitar.
# O par vai cifrado e com a validade dentro da mensagem: quem lê o cache não vê token, e valor adulterado, de outra
# chave ou vencido é ignorado, como se não houvesse memo.
class RefreshingMarkuppApi
  RENEWED_PAIR_TTL = 30.seconds
  # Travas do processo, valem para as threads do Puma. Mais réplicas exigem sessão fixa no Gateway (ver config/puma.rb).
  # ponytail: a trava fica presa durante o POST /auth/refresh e são 64 faixas pelo digest; sessões da mesma faixa
  # esperam umas pelas outras, até o MarkuppApi::READ_TIMEOUT.
  RENEWAL_LOCKS = Array.new(64) { Mutex.new }

  PAIR_KEY_SALT = "panel/renewed_pair"

  def initialize(credential, renewed_pairs: Rails.cache)
    @credential = credential
    @renewed_pairs = renewed_pairs
  end

  # key_generator do Rails guarda a chave derivada: não refaz o PBKDF2 a cada requisição.
  def self.pair_encryptor
    key = Rails.application.key_generator.generate_key(PAIR_KEY_SALT, ActiveSupport::MessageEncryptor.key_len)
    ActiveSupport::MessageEncryptor.new(key)
  end

  def get(path) = with_fresh_token("GET #{path}") { it.get(path) }

  def conditional_get(path, etag:) = MarkuppApi::Revision.new(body: get(path), etag: nil)

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
      key = "panel/renewed_pair/#{digest}"
      RENEWAL_LOCKS[digest.hex % RENEWAL_LOCKS.size].synchronize do
        remembered_pair(key) || remember_pair(key, @credential.public_api.post("/auth/refresh", refresh_token:))
      end
    end

    def remembered_pair(key)
      sealed = @renewed_pairs.read(key)
      self.class.pair_encryptor.decrypt_and_verify(sealed, purpose: key) if sealed.is_a?(String)
    rescue ActiveSupport::MessageEncryptor::InvalidMessage
      nil
    end

    def remember_pair(key, pair)
      sealed = self.class.pair_encryptor.encrypt_and_sign(pair, purpose: key, expires_in: RENEWED_PAIR_TTL)
      @renewed_pairs.write(key, sealed, expires_in: RENEWED_PAIR_TTL)
      pair
    end
end
