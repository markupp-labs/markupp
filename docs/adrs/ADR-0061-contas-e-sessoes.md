# ADR-0061: Sessões como entidade, bloqueio pelo administrador da instância e histórico com prazo

## Status

Proposta

## Contexto

O administrador da instância (ADR-0057) não vê quem usa a instância nem consegue cortar o acesso
de uma conta ou de um cliente sem ir ao banco. O JWT de acesso não guarda estado e a API não o
confere no banco. Cada renovação do painel cria um refresh sem ligação com o anterior, e nem ele
nem os tokens OAuth guardam IP, cliente ou último uso. O painel chama a API de servidor para
servidor, então a API vê o IP do painel, não o de quem usa

## Decisão

O login abre uma sessão, que agrupa os refresh das renovações e guarda o IP, o user agent, a
criação e o último uso. A autorização OAuth guarda o mesmo. O último uso muda no login, em cada
renovação e em cada troca de token do MCP, nunca a cada requisição. O painel repassa o IP e o user
agent do navegador no login e na renovação, e a API só confia neles quando o painel está em
`MARKUPP_TRUSTED_PROXIES`, a mesma regra do limitador. Os clientes OAuth (MCP, CLI, plugin, apps
de terceiros) têm a origem do pedido ao `/oauth/token`, e o servidor MCP repassa o IP do agente na
troca, como já faz na leitura pública. O JWT leva o id da sessão no claim `sid`, só para a lista
marcar a sessão atual

A pessoa tem dois recursos, como nos grandes provedores: as sessões de login, em `/v1/me/sessions`,
e os apps autorizados, em `/v1/me/authorizations`. O token OAuth só chega às sessões com o escopo
admin. As rotas do administrador ficam sob `/v1/instance`, com o mesmo escopo e as mesmas
ferramentas no MCP. A conta bloqueada recebe `account_blocked` só depois de a senha ou o provedor
conferirem, e no OAuth o `invalid_grant` da RFC 6749

O administrador lista as contas e as sessões e clientes de cada uma. Ele bloqueia uma conta, que
deixa de entrar por qualquer método e perde as sessões e as autorizações, e as notas ficam até ele
desbloquear ou apagar a conta. Apagar a conta apaga junto os tenants em que ela é a única owner,
com cofres e notas, e o painel avisa antes quantos tenants e notas somem; nos outros tenants a
conta só sai. Ele também encerra uma sessão, revoga um cliente OAuth de uma conta e bloqueia um
IP ou uma faixa. O bloqueio de IP fica no banco, vale antes da autenticação em toda rota da API, e
cada réplica relê a lista em alguns segundos. A pessoa vê e encerra as próprias sessões e
clientes em Configurações > Conta

O corte vale pela renovação: o JWT já emitido vale até expirar, sem consulta ao banco por
requisição. O padrão de `MARKUPP_JWT_EXPIRATION_MINUTES` cai de 15 para 5 minutos, o mesmo prazo do
JWT que o MCP troca

O histórico de acessos e de mudanças sensíveis é o `audit_events`, que passa a guardar IP e user
agent, e o administrador o lê pela API. Um prazo só vale para o histórico e para o IP, e o padrão
é 365 dias. O administrador escolhe o prazo, a escolha fica no banco, e com
`MARKUPP_ACCESS_HISTORY_RETENTION_DAYS` definida vale a variável, como no ADR-0057. Uma limpeza
diária apaga o histórico que passou do prazo e tira o IP e o user agent das sessões encerradas há
mais tempo que ele; as sessões ativas guardam os dois enquanto estão ativas. Só a limpeza apaga o histórico; as outras alterações continuam proibidas

## Alternativas consideradas

- Conferir a sessão no banco a cada requisição: o corte é imediato, mas cada chamada passa pelo banco
- Lista de bloqueios em memória relida por réplica: o corte leva segundos, mas é mais um estado
  a manter em cada réplica para algo que o JWT curto já resolve
- Manter o JWT em 15 minutos: o bloqueio leva até 15 minutos para valer
- Sessão do painel sem IP: o administrador não distingue um acesso do painel de outro
- Bloquear IP só no login e na renovação: o que já entrou segue lendo até o JWT expirar
- Prazo de guarda só na variável: quem hospeda pelo painel não ajusta sem reiniciar
- Prazos separados para o IP e para o histórico: mais uma escolha para quem hospeda, sem pedido
  de quem usa
- Guardar para sempre sem escolha: o IP fica guardado sem prazo em toda instalação que não mexer
- Só tirar o IP e manter as linhas do histórico: o histórico cresce sem limite
- Recusar apagar a conta enquanto ela for a única owner de algum tenant: o administrador não tira
  da instância uma conta abusiva sem passar tenant por tenant
- Promover outro membro a owner ao apagar a conta: alguém herda notas que não escolheu receber

## Consequências

- Depois do bloqueio, a conta ou a sessão ainda acessa por até 5 minutos, ou pelo prazo do JWT que
  quem hospeda escolheu
- O painel e o MCP renovam o JWT com mais frequência
- Atrás de um proxy fora de `MARKUPP_TRUSTED_PROXIES`, as sessões do painel mostram o IP do painel
- IP e histórico somem depois do prazo, e o administrador perde o que era mais antigo
