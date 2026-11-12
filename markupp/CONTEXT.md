# Markupp

O servidor que guarda as notas, deriva a organização a partir delas e expõe tudo por uma API
REST versionada.

## Language

**Cliente**:
Tudo que consome o markupp, pela API REST ou pelo servidor MCP. Painel web, plugin de editor
e agente de IA são clientes, e nenhum deles é privilegiado. O navegador consome o painel, não
o markupp.
_Avoid_: consumidor, integração, aplicação externa, frontend

**Self-host**:
Instalar e operar o markupp por conta própria. É o produto completo, sem recurso reservado.
_Avoid_: on-premise, comunidade, open source

**Nota**:
A unidade de conteúdo markdown que o cliente cria, lê, edita e apaga.
_Avoid_: anotação, documento, arquivo, página

**Caminho**:
O identificador lógico da nota, único dentro do cofre a que ela pertence.
_Avoid_: path, rota, diretório, pasta

**Título**:
O nome pelo qual uma nota é citada: o último segmento do caminho sem a extensão, mais os
apelidos que a própria nota declara no frontmatter. Títulos que diferem só em caixa ou em
forma Unicode são o mesmo título.
_Avoid_: nome, heading, cabeçalho, title

**Ligação**:
A referência que uma nota faz a outra do mesmo cofre, escrita no conteúdo como wikilink ou
link markdown. Ela aponta para um título e liga a todas as notas que têm esse título.
_Avoid_: link quando se refere ao conceito, referência, relação, aresta

**Ligação não resolvida**:
A ligação cujo título não é de nenhuma nota do cofre.
_Avoid_: link quebrado, link morto, órfã

**Ligação inferida**:
A ligação que o servidor deriva para uma ligação não resolvida, apontando para a nota mais
provável. Ela existe só no plano de recuperação e nunca é escrita na nota.
_Avoid_: correção, autocorreção, ligação sugerida

**Ligação tipada**:
A ligação declarada no frontmatter, em que a chave dá o tipo da relação entre a nota e o
título citado.
_Avoid_: propriedade, metadado, relação quando se refere só ao frontmatter

**Ligação automática**:
A ligação que o servidor cria entre duas notas sem que o usuário tenha escrito ligação
nenhuma entre elas. É por menção, por texto parecido ou por sentido, e entre duas notas há
no máximo uma. Ela existe só no plano de recuperação, nunca é escrita na nota e pesa menos
que a ligação escrita.
_Avoid_: ligação inferida, ligação sugerida, link implícito

**Ligação por menção**:
A ligação automática da nota que cita o título de outra no texto para a nota citada.
_Avoid_: menção quando se refere à ligação, backlink não ligado

**Ligação por texto parecido**:
A ligação automática entre duas notas cujo texto é parecido, cada uma entre as mais
parecidas da outra.
_Avoid_: ligação relacionada, sugestão quando se refere à ligação

**Ligação por sentido**:
A ligação automática entre duas notas que tratam do mesmo assunto, mesmo sem dividir
palavras, cada uma entre as mais próximas da outra.
_Avoid_: ligação semântica, ligação por embedding

**Recusa de ligação**:
A decisão do usuário de que duas notas não devem ter ligação automática entre elas, de
nenhum tipo. Fica no plano de registro, vale até ser desfeita e o servidor a respeita a cada
vez que refaz as ligações automáticas.
_Avoid_: remoção, exclusão de ligação, bloqueio

**Menção não ligada**:
O título de uma nota citado no texto de outra sem que ali exista ligação escrita, inferida ou
automática, porque o título é comum ou porque o par foi recusado.
_Avoid_: citação, referência implícita, backlink não ligado

**Nota órfã**:
A nota sem nenhuma ligação, nem de entrada nem de saída, contando as inferidas e as
automáticas.
_Avoid_: nota solta, nota isolada, ligação não resolvida

**Agrupamento**:
Um conjunto de notas do mesmo cofre reunidas por tema, sem o usuário pedir. Um agrupamento
nunca atravessa cofre.
_Avoid_: pasta, categoria, tag

**Comunidade**:
O agrupamento derivado da estrutura das ligações: notas mais ligadas entre si do que com o
resto do cofre.
_Avoid_: cluster, tema quando se refere a este tipo

**Cofre**:
O namespace e a unidade de acesso. A nota pertence a exatamente um cofre, e quem é membro do
cofre lê e escreve todas as notas dele, salvo o viewer, que só lê. O membro entra adicionado
por outro membro, entre as contas que já têm vínculo com o tenant.
_Avoid_: workspace, projeto, biblioteca, espaço

**Nota pública**:
A nota que qualquer pessoa, com ou sem conta, lê pelo endereço público, porque foi publicada
sozinha ou está numa pasta ou cofre publicado. Ninguém escreve nela por esse caminho.
_Avoid_: página pública, site, nota compartilhada

**Publicação**:
O ato de tornar pública uma nota, uma pasta ou um cofre, e o registro desse ato, com autor e
data. Despublicar desfaz a publicação, e as notas voltam a ser lidas só pelos membros.
_Avoid_: compartilhamento, exportação, deploy

**Tenant**:
A unidade de isolamento dentro do banco. Toda linha de dado carrega o tenant a que
pertence. A conta entra num tenant por vínculo.
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
admin administram o tenant, só o owner apaga o tenant, e o viewer só lê nos cofres de que é
membro, sem gerenciar cofre, mas pode sair dele. O acesso às notas continua sendo pelo cofre.
_Avoid_: associação, assinatura, membership

**Convite**:
O pedido de vínculo que owner ou admin emite para um email, com papel e validade. A conta
desse email aceita ou recusa; quem ainda não tem conta se cadastra pelo token do convite.
_Avoid_: cadastro, convocação, autorregistro

**Notificação**:
O que pede atenção da conta no painel, como um convite pendente. É da conta, não do tenant, e
some quando a conta resolve o que a originou.
_Avoid_: alerta, aviso, email

**Autorização**:
O consentimento de uma conta para um cliente agir em nome dela. A conta pode revogar a
autorização.
_Avoid_: permissão, grant, acesso

**Escopo**:
O que uma Autorização permite ao cliente: ler notas, escrever notas ou administrar.
_Avoid_: permissão, papel

**Pedido de autorização**:
A solicitação de um cliente para agir em nome da conta. A conta aprova ou recusa, e o pedido
expira se ninguém responder.
_Avoid_: solicitação de acesso, convite
