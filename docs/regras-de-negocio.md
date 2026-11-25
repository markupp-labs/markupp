# Regras de negócio

## Tenant e papéis

A conta entra num tenant por vínculo, com um papel: owner, admin, member ou viewer.

Qualquer conta cria um tenant e vira owner dele.

Owner e admin convidam, trocam papel e removem membros. Ninguém concede papel acima do seu nem
altera quem tem papel acima do seu.

O tenant nunca fica sem owner, e só o owner apaga o tenant.

## Convite

O convite é para um email, com papel e validade de uma semana, e só owner ou admin o emite.

Com SMTP, o convite sai por email e só a conta de email verificado convida. Sem SMTP, a API
devolve o token para ser repassado à mão.

Cada conta e cada tenant têm um teto de convites por janela de 24 horas, ajustável em
`MARKUPP_INVITE_LIMIT_PER_ACCOUNT` e `MARKUPP_INVITE_LIMIT_PER_TENANT`. Convite revogado
continua contando.

Quem não tem conta aceita pelo token e escolhe a senha, e a conta nasce com o email do convite
já verificado, mesmo com o cadastro fechado.

A conta que já existe só vê e aceita o convite endereçado ao email dela depois de verificá-lo.

Toda conta recebe um cofre ao entrar num tenant, e esse cofre é comum como qualquer outro.

## Cofre e acesso

A nota pertence a exatamente um cofre.

Quem é membro de um cofre lê e escreve todas as notas dele, salvo o viewer, que só lê. Não
existe permissão por nota.

O viewer do tenant pode ser membro de cofre e só lê: não escreve nota, não cria, não renomeia
nem apaga cofre, não adiciona nem remove membro e não publica.

Qualquer usuário que não é viewer cria cofre, e qualquer membro que não é viewer renomeia e
apaga o cofre e adiciona e remove membro, inclusive quem criou o cofre.

O membro adicionado é uma conta que já tem vínculo com o tenant. Quem é de fora entra antes
pelo convite.

Qualquer membro do tenant vê a lista de colegas do tenant para escolher quem adicionar ao
cofre.

Qualquer membro, viewer inclusive, sai do cofre removendo a si mesmo.

O nome do cofre não é único no tenant.

Apagar um cofre apaga as notas dele e todo o dado derivado delas.

Um cofre pode acabar sem nenhum membro, e uma conta pode ficar sem nenhum cofre.

Nenhuma consulta autenticada atravessa tenant, e nenhuma alcança cofre de que o usuário não é
membro.

## Nota

O caminho da nota é único dentro do cofre.

O servidor nunca altera o conteúdo nem o caminho de uma nota.

Ligação entre notas não atravessa cofre.

Escrita sobre uma nota falha quando a versão enviada não é a corrente, a menos que o cliente
peça sobrescrita.

## Publicação

O alvo é uma nota, uma pasta ou o cofre inteiro. Pasta e cofre publicados incluem as notas que
entrarem neles depois, e a nota que sai da pasta publicada deixa de ser pública.

Quem pode editar a nota a publica. Pasta e cofre, só owner e admin publicam.

Membro do cofre que não é viewer despublica a nota publicada sozinha, e owner e admin
despublicam qualquer publicação do tenant.

Publicar exige o nome do alvo digitado como confirmação.

A nota pública tem o endereço `/p/{titulo}-{id}`, com id público criado ao publicar. O id
interno da nota não aparece. Despublicar invalida o id, e publicar de novo gera outro.

A página pública sai com `noindex`, salvo quando quem publica libera a indexação por buscadores
naquela publicação.

A leitura pública não pede conta, nunca escreve e só alcança notas publicadas, inclusive na
busca e no grafo públicos. O banco garante isso por um papel próprio, sem acesso a contas,
tenants, convites nem auditoria.

A leitura pública tem limite de pedidos por IP.

## Organização derivada

Índice, grafo de ligações, agrupamento e vetores derivam das notas e são reconstruíveis a
qualquer momento.

O plano de recuperação observa o plano de registro e nunca escreve de volta.

O índice pode ficar atrás das notas, e ficar atrás não impede leitura nem escrita.

O dado derivado é calculado por cofre, então tema que atravessa cofres não forma agrupamento.

Entre duas notas há no máximo uma ligação automática, e nenhuma quando já existe ligação
escrita ou inferida entre elas.

A recusa de ligação vale para o par nos dois sentidos e para qualquer tipo de ligação
automática, até ser desfeita. O viewer não recusa.

## Busca

A busca só retorna nota de cofre de que o usuário é membro.

A busca é léxica, e o título pesa mais que o corpo.

## Embedding

Uma instância usa um modelo de embedding por vez, e trocar de modelo apaga os vetores e refaz
os de todas as notas.

Sem provedor de embedding, não existe ligação por sentido, e o resto da organização derivada
continua completo.

## Autenticação e cadastro

O acesso exige autenticação, por email e senha ou por provedor de login externo ligado na
instância. Quem hospeda pode desligar o login por senha em `MARKUPP_AUTH_LOCAL_ENABLED`.

O login nunca cria conta. Email desconhecido é recusado como senha errada.

A identidade externa só entra na conta a que está vinculada. O vínculo nasce de uma sessão
aberta, nunca pelo email que o provedor informa.

A conta não desvincula a última forma de entrar que tem.

A primeira conta da instância sempre nasce, e vira owner do tenant inicial.

As outras contas nascem só por método de cadastro aberto. O interruptor geral,
`MARKUPP_AUTH_REGISTRATION_ENABLED`, vem desligado; desligado, nenhum método cadastra e só
entra quem for convidado.

Com o interruptor geral ligado, cada método, email e senha e cada provedor, fica aberto até o
administrador da instância fechá-lo. A variável do método, quando definida, vence a escolha do
administrador.

Fechar o cadastro de um método não fecha o login por ele.

A conta tem chave estável própria, e o identificador do provedor fica guardado à parte, para
que trocar de provedor ou de email não perca o vínculo com o histórico.

## Administração da instância

Administra a instância a conta de email verificado listada em `MARKUPP_INSTANCE_ADMINS`.
Sem a lista, ninguém administra pelo painel.

O administrador da instância escolhe o provedor de embedding, mantém os provedores de login
externo e abre ou fecha o cadastro por método. Em cada um, a variável de ambiente de quem
hospeda vence e trava a escolha.

Segredo guardado no banco, como chave de API e client secret, fica cifrado e nenhuma resposta o
devolve.

O id de um provedor de login não muda depois de criado, e o issuer não muda enquanto houver
conta vinculada. Apagar o provedor apaga os vínculos das contas com ele.

## Autorização de clientes

O cliente OAuth age em nome da conta só depois que ela aprova o pedido de autorização, e só
dentro dos escopos aprovados: ler notas, escrever notas ou administrar.

A conta revoga a autorização quando quiser.

## Email e notificação

O envio de email sai do caminho da requisição.

Sem SMTP, o sistema não envia, e a verificação de email e a recuperação de acesso por senha
ficam indisponíveis.

O convite pendente aparece como notificação no painel da conta convidada.

## Auditoria

Operações críticas, como login, convite, mudança de vínculo, gestão de cofre, publicação e
autorização OAuth, gravam uma linha de auditoria na mesma transação.

A linha registra tenant, ator, ação, alvo, momento e resultado.

O banco recusa atualizar ou excluir linha de auditoria.
