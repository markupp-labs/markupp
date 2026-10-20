# ADR-0038: Servidor MCP em TypeScript, cliente da API

## Status

Aceita

## Contexto

O ADR-0028 prevê o servidor MCP como serviço próprio, e o ADR-0026 o coloca como cliente par
da API REST. Faltava decidir a linguagem, o transporte e o formato das ferramentas. Os SDKs de
MCP mais completos são os de TypeScript, enquanto a lógica de notas, isolamento e auditoria já
mora no servidor Go

## Decisão

O servidor MCP é escrito em TypeScript com FastMCP, mantido neste repositório, e consome o
servidor Go só pela API `/v1`. Atende por Streamable HTTP, num Deployment próprio em
subdomínio separado, e por stdio, distribuído no pacote npm `markupp-mcp`. As ferramentas são
orientadas a tarefa: buscar, ler, criar, editar por troca de trecho e apagar notas, e uma
ferramenta por operação de administração de tenants, membros e convites. O tenant é parâmetro
de cada ferramenta, e as notas também são expostas como resources

## Alternativas consideradas

- Servidor MCP em Go, dentro do binário do markupp: uma toolchain só, com SDK menos maduro e
  tentação de pular a API e chamar o storage direto
- Código TypeScript chamando o Go por fora da API (banco compartilhado, gRPC interno ou
  subprocesso): contornaria o contrato do ADR-0026 e o isolamento por RLS do ADR-0016
- Ferramentas espelhando a API endpoint a endpoint: geração mecânica, ao custo de mais
  descrições no contexto do modelo e de chamadas encadeadas pelo agente
- Tenant fixo por conexão: menos parâmetros, e um agente não atuaria em dois tenants na mesma
  conexão

## Consequências

- O CI passa a rodar Go e Node, e Ruby também quando o painel do ADR-0031 entrar
- Os testes usam um fake nomeado do cliente da API, e um teste ponta a ponta contra o Go real
  no compose pega quebra de contrato no mesmo CI
- A edição por trecho lê e grava em duas chamadas, e manda no PUT o `lastModifiedAt` lido para
  que uma escrita alheia no meio vire 409 em vez de ser sobrescrita
- Enquanto a busca da API for por substring, a ferramenta de busca devolve só caminhos e o
  agente lê as notas que interessarem
- Ferramentas de cofre entram quando as rotas de cofre existirem, e até lá a criação de nota
  usa o cofre padrão
- O prefixo `/v1` do ADR-0026 precisa sair antes do servidor MCP
