# Segura cada POST /auth/refresh da FakeMarkuppAuth até `callers` chamadas chegarem juntas, ou até o timeout.
# Faz o papel da API lenta: com o Net::HTTP de verdade a thread solta a GVL e a outra requisição entra no meio.
class FakeRefreshGate
  def initialize(callers, timeout:)
    @callers = callers
    @timeout = timeout
    @arrived = 0
    @lock = Mutex.new
    @all_arrived = ConditionVariable.new
  end

  def pass
    @lock.synchronize do
      @arrived += 1
      @all_arrived.broadcast
      wait_for_the_others
    end
  end

  private
    def wait_for_the_others
      deadline = monotonic_now + @timeout
      @all_arrived.wait(@lock, deadline - monotonic_now) while @arrived < @callers && monotonic_now < deadline
    end

    def monotonic_now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
end
