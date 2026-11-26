require "test_helper"

# Configurações da instância, seção Acessos (ADR-0061): os IPs e faixas bloqueados e o prazo de guarda do histórico.
module Settings
  class InstanceAccessControllerTest < ActionDispatch::IntegrationTest
    ADMIN_ACCOUNTS = [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ].freeze

    def sign_in_admin(**scenario)
      fake = use_fake_api(accounts: ADMIN_ACCOUNTS, instance_access: scenario)
      sign_in_as
      fake
    end

    test "the access section lists the blocked ranges and the retention" do
      sign_in_admin(ip_blocks: [ { id: "b1", cidr: "198.51.100.0/24", note: "scan" } ], retention: { days: 90 })

      get settings_instance_access_path

      assert_select "main h1", "Access"
      assert_select "main nav a[href=?][aria-current=page]", settings_instance_access_path
      assert_select "#ip_blocks li[data-ip-block=b1]", text: /198\.51\.100\.0\/24.*scan/m do
        assert_select "form[action=?] button", settings_instance_ip_block_path("b1"), text: "Unblock"
      end
      assert_select "form[action=?] input[name='retention[days]'][value='90']", settings_instance_retention_path
    end

    test "blocking a range adds it" do
      fake = sign_in_admin

      post settings_instance_ip_blocks_path, params: { ip_block: { cidr: "198.51.100.0/24", note: "scan" } }

      assert_redirected_to settings_instance_access_path
      assert_equal "198.51.100.0/24 blocked. Every replica refuses it within seconds.", flash[:notice]
      assert_equal [ "198.51.100.0/24" ], fake.instance_access.ip_blocks.map { it["cidr"] }
    end

    test "a range that is not an IP or that holds the admin's own IP is refused with the reason" do
      sign_in_admin

      post settings_instance_ip_blocks_path, params: { ip_block: { cidr: "rede" } }
      assert_equal "Use an IP or a CIDR range, such as 198.51.100.7 or 198.51.100.0/24.", flash[:alert]

      post settings_instance_ip_blocks_path, params: { ip_block: { cidr: "0.0.0.0/0" } }
      assert_equal "That range holds your own IP, and blocking it would lock you out.", flash[:alert]
    end

    test "unblocking removes the range" do
      fake = sign_in_admin(ip_blocks: [ { id: "b1", cidr: "198.51.100.0/24" } ])

      delete settings_instance_ip_block_path("b1")

      assert_redirected_to settings_instance_access_path
      assert_empty fake.instance_access.ip_blocks
    end

    test "changing the retention saves it" do
      sign_in_admin

      patch settings_instance_retention_path, params: { retention: { days: "30" } }

      assert_redirected_to settings_instance_access_path
      assert_equal "History and IPs are now kept for 30 days.", flash[:notice]
    end

    test "a retention out of range is refused" do
      sign_in_admin

      patch settings_instance_retention_path, params: { retention: { days: "0" } }

      assert_equal "Keep them for 1 to 3650 days.", flash[:alert]
    end

    test "a retention fixed by the host shows read only" do
      sign_in_admin(retention: { days: 30, host_locked: true })

      get settings_instance_access_path

      assert_select "#retention_locked", text: /MARKUPP_ACCESS_HISTORY_RETENTION_DAYS/
      assert_select "input[name='retention[days]']", count: 0
    end
  end
end
