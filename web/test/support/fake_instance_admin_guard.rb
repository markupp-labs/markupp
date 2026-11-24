# Como a API nas rotas da instância: só a conta com instance_admin passa.
module FakeInstanceAdminGuard
  private
    def refuse_non_admin(account, call)
      return if account&.fetch("instance_admin", false)

      raise MarkuppApi::InstanceAdminRequired, "fake API refuses #{call}: instance_admin_required, expected an instance admin account"
    end
end
