# ADR-0051: Servidor MCP público para cofres publicados

## Status

Proposta

## Contexto

O servidor MCP do ADR-0045 exige Autorização OAuth em qualquer transporte. Um cofre público
(ADR-0048) deve ser lido sem conta também por agentes, e a documentação do markupp,
publicada como cofre (ADR-0048), deve ser lida por agentes tão facilmente quanto por pessoas

## Decisão

O servidor MCP expõe um segundo endpoint, `/mcp/public`, sem OAuth, com ferramentas só de
leitura sobre cofres publicados: listar, ler e buscar notas por slug. Essas ferramentas chamam
apenas as rotas `/v1/public/...` da API, herdando o papel `markupp_reader`, o limite por IP e o
404 para cofre não publicado. O endpoint autenticado continua como está, e as ferramentas dos
dois nunca se misturam no mesmo registro.

O site também serve a documentação em formato que agentes leem sem MCP: `llms.txt`,
`llms-full.txt` e cada página em markdown cru

## Alternativas consideradas

- Ferramentas públicas no mesmo endpoint autenticado: obrigaria o cliente a passar por OAuth
  para ler o que é público
- Só `llms.txt`, sem MCP: atende leitura estática, mas perde a busca e a navegação por notas
- Token anônimo emitido pelo authorization server: soma estado e revogação para um acesso que
  não precisa de identidade

## Consequências

- Há uma segunda superfície sem autenticação, protegida pelos mesmos limites da API pública
- O servidor MCP não ganha acesso ao banco nem papel novo: continua cliente da API
