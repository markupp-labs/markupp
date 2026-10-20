# Markupp

O servidor que guarda as notas, deriva a organização a partir delas e expõe tudo por uma API
REST versionada.

## Language

**Cliente**:
Tudo que consome o markupp, pela API REST ou pelo servidor MCP. Painel web, plugin de editor
e agente de IA são clientes, e nenhum deles é privilegiado. O navegador consome o painel, não
o markupp.
_Avoid_: consumidor, integração, aplicação externa, frontend

**Enterprise**:
A edição que a equipe opera, com vários tenants. O nome diz quem opera, não onde roda.
_Avoid_: nuvem, SaaS, hospedado

**Self-host**:
A edição que qualquer pessoa instala e opera por conta própria, com um tenant só e sem
plano de controle.
_Avoid_: on-premise, comunidade, open source

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
A unidade de isolamento dentro do banco. Toda linha de dado carrega o tenant a que
pertence, e o self-host roda com um só. A conta entra num tenant por vínculo.
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

**Identidade**:
O vínculo entre uma credencial (email e senha ou provedor externo) e a conta de um usuário.
A identidade é única globalmente pelo par provedor e identificador do provedor.
_Avoid_: login, credencial avulsa, perfil

**Conta**:
A pessoa na instância, com email único, que participa de vários tenants. Login e sessão são
da conta; o tenant em que ela age vem de cada requisição.
_Avoid_: usuário do tenant, perfil

**Vínculo**:
A participação de uma conta num tenant, com um papel: owner, admin, member ou viewer. Owner e
admin administram o tenant, só o owner apaga o tenant, e o viewer só lê nos cofres de que é membro. O acesso às notas
continua sendo pelo cofre.
_Avoid_: associação, assinatura, membership

**Convite**:
O pedido de vínculo que owner ou admin emite para um email, com papel e validade. A conta
desse email aceita ou recusa; quem ainda não tem conta se cadastra pelo token do convite.
_Avoid_: cadastro, convocação, autorregistro
