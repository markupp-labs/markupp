# Apps rack de mentira para o PanelExceptionsApp: uma que quebra no meio da página e outra que só guarda o env recebido.
class FakeCrashingRackApp
  def call(env) = raise("fake error page broke on #{env['PATH_INFO']}")
end

class FakeRecordingRackApp
  attr_reader :received_env

  def initialize(response) = @response = response

  def call(env)
    @received_env = env
    @response
  end
end
