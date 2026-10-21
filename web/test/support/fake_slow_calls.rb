# A API lenta dos testes do indicador de carregamento: segura as chamadas da FakeMarkuppApi que começam com um prefixo
# ("GET /notes/2") até o teste soltar. Sem sleep: o teste decide quando a resposta chega. O timeout só evita prender o
# servidor do Capybara se o teste esquecer de soltar.
class FakeSlowCalls
  def initialize(timeout:)
    @timeout = timeout
    @held_prefixes = []
    @lock = Mutex.new
    @released = ConditionVariable.new
  end

  def hold(call_prefix) = @lock.synchronize { @held_prefixes << call_prefix }

  def release
    @lock.synchronize do
      @held_prefixes.clear
      @released.broadcast
    end
  end

  def pass(call)
    deadline = monotonic_now + @timeout
    @lock.synchronize do
      @released.wait(@lock, deadline - monotonic_now) while held?(call) && monotonic_now < deadline
    end
  end

  private
    def held?(call) = @held_prefixes.any? { call.start_with?(it) }

    def monotonic_now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
end
