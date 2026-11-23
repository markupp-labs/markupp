# Faz o papel da PublicRelayCredential: devolve sempre o HTTP Basic dado.
class FakePublicRelay
  def initialize(authorization) = @authorization = authorization

  attr_reader :authorization
end
