# ADR-0050: Servidor MCP público para notas públicas

## Status

Proposta

## Contexto

O servidor MCP do ADR-0045 exige Autorização OAuth em qualquer transporte. Uma nota pública
(ADR-0048) deve ser lida sem conta também por agentes, tão facilmente quanto por pessoas

## Decisão

O servidor MCP expõe um segundo endpoint, `/mcp/public`, sem OAuth, com ferramentas só de
leitura sobre notas públicas: listar, ler e buscar pelo id público. Essas ferramentas chamam
apenas as rotas `/v1/public/...` da API, herdando o papel `markupp_reader`, o limite por IP e o
404 para nota não publicada. O endpoint autenticado continua como está, e as ferramentas dos
dois nunca se misturam no mesmo registro

## Alternativas consideradas

- Ferramentas públicas no mesmo endpoint autenticado: obrigaria o cliente a passar por OAuth
  para ler o que é público
- Só um arquivo estático para agentes, como `llms.txt`: atende leitura estática, mas perde a busca e a navegação por notas
- Token anônimo emitido pelo authorization server: soma estado e revogação para um acesso que
  não precisa de identidade

## Consequências

- Há uma segunda superfície sem autenticação, protegida pelos mesmos limites da API pública
- O servidor MCP não ganha acesso ao banco nem papel novo: continua cliente da API
