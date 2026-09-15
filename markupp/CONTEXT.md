# Markupp

O servidor que guarda as notas, deriva a organização a partir delas e expõe tudo por uma API
REST versionada.

## Language

**Cliente**:
Tudo que consome o markupp, pela API REST ou pelo servidor MCP. Navegador com o painel web,
plugin de editor e agente de IA são clientes, e nenhum deles é privilegiado.
_Avoid_: consumidor, integração, aplicação externa, frontend

**Nota**:
A unidade de conteúdo markdown que o cliente cria, lê, edita e apaga.
_Avoid_: anotação, documento, arquivo, página

**Caminho**:
O identificador lógico da nota, único dentro do cofre a que ela pertence.
_Avoid_: path, rota, diretório, pasta

**Cofre**:
O namespace e a unidade de acesso. A nota pertence a exatamente um cofre, e quem é membro do
cofre lê e escreve todas as notas dele.
_Avoid_: workspace, projeto, biblioteca, espaço

**Tenant**:
A unidade de isolamento dentro do banco. Toda linha carrega o tenant a que pertence, e o
self-host roda com um só.
_Avoid_: cliente, organização, conta

**Plano de registro**:
Onde as notas vivem. É durável e é a fonte da verdade do sistema.
_Avoid_: banco principal, armazenamento, write model

**Plano de recuperação**:
Tudo que é derivado das notas e reconstruível a qualquer momento: índice léxico, vetores,
grafo de ligações e agrupamentos.
_Avoid_: cache, índice quando se refere ao conjunto inteiro

**Organização derivada**:
A estrutura que o servidor calcula a partir das notas sem alterar conteúdo nem caminho. É o
mecanismo por trás da base auto-organizável.
_Avoid_: hierarquia, taxonomia, classificação automática

**Indexador**:
O serviço que lê o plano de registro e escreve o plano de recuperação. São dois, o
determinístico e o de embedding.
_Avoid_: worker, processador, pipeline
