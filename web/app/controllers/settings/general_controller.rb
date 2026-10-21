# Seção General do tenant ativo, como a de um repositório no GitHub: o nome para todo papel e a Danger zone para o owner.
module Settings
  class GeneralController < BaseController
    skip_before_action :pick_tenant
    before_action :require_tenant

    def show
    end
  end
end
