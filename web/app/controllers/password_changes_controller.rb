# Troca de senha de quem está logado: a API só troca pelo token do email, então pede o email de redefinição
# para a própria conta, pela mesma rota do esqueci a senha.
class PasswordChangesController < ApplicationController
end
