# Fica no lugar do MarkuppApi nos testes, sem rede: repassa cada chamada à fake do assunto.
class FakeMarkuppApi
  delegate :put_requests, :post_requests, to: :@notes

  def initialize(notes: [], unreachable: false, conflict: false)
    @notes = FakeMarkuppNotes.new(notes:, conflict:)
    @unreachable = unreachable
  end

  def get(path) = receive("GET", path)

  def put(path, body) = receive("PUT", path, body)

  def post(path, body) = receive("POST", path, body)

  def delete(path) = receive("DELETE", path)

  private
    def receive(verb, path, *body)
      raise MarkuppApi::Error, "fake API unreachable on #{verb} #{path}" if @unreachable

      fake_for(verb, path).public_send(verb.downcase, path, *body)
    end

    def fake_for(verb, path)
      return @notes if path.start_with?("/notes")

      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected /notes"
    end
end
