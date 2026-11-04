-- Troca para o papel sem privilégio e entrega ao RLS o tenant, o usuário e o papel
-- dele no tenant. O último argumento true limita tudo à transação, como SET LOCAL.
-- name: EnterActor :exec
SELECT set_config('role', 'markupp_app', true),
       set_config('markupp.tenant_id', sqlc.arg(tenant_id)::text, true),
       set_config('markupp.user_id', sqlc.arg(user_id)::text, true),
       set_config('markupp.role', sqlc.arg(role)::text, true);

-- Troca para o papel da leitura sem conta (ADR-0048), que só enxerga os cofres
-- publicados. Zera tenant, usuário e papel, porque as políticas antigas de notes valem
-- para todos os papéis e a conexão do pool pode trazê-los da sessão.
-- name: EnterPublicReader :exec
SELECT set_config('role', 'markupp_reader', true),
       set_config('markupp.tenant_id', '', true),
       set_config('markupp.user_id', '', true),
       set_config('markupp.role', '', true);
