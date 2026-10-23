# Regras de negócio

## Cofre e acesso

A nota pertence a exatamente um cofre.

Quem é membro de um cofre lê e escreve todas as notas dele, salvo o viewer, que só lê. Não
existe permissão por nota.

O viewer do tenant pode ser membro de cofre e só lê: não escreve nota, não cria, não renomeia
nem apaga cofre e não adiciona nem remove membro.

Qualquer usuário que não é viewer cria cofre, e qualquer membro que não é viewer renomeia e
apaga o cofre e adiciona e remove membro, inclusive quem criou o cofre.

O membro adicionado é uma conta que já tem vínculo com o tenant. Quem é de fora entra antes
pelo convite do tenant.

Qualquer membro do tenant vê a lista de colegas do tenant para escolher quem adicionar ao
cofre.

Qualquer membro, viewer inclusive, sai do cofre removendo a si mesmo.

O nome do cofre não é único no tenant.

Apagar um cofre apaga as notas dele e todo o dado derivado delas.

Um cofre pode acabar sem nenhum membro, e uma conta pode ficar sem nenhum cofre.

Todo usuário recebe um cofre ao entrar num tenant, e esse cofre é comum como qualquer outro.

Mover uma nota entre cofres é apagar na origem e criar no destino, então ela recebe
identificador novo e não leva histórico junto.

Nenhuma consulta atravessa tenant, e nenhuma alcança cofre de que o usuário não é membro.

## Nota

O caminho da nota é único dentro do cofre.

O servidor nunca altera o conteúdo nem o caminho de uma nota.

Ligação entre notas não atravessa cofre.

Escrita sobre uma nota falha quando a versão enviada não é a corrente, a menos que o cliente
peça sobrescrita.

## Organização derivada

Índice, grafo de ligações, agrupamento e vetores derivam das notas e são reconstruíveis a
qualquer momento.

O plano de recuperação observa o plano de registro e nunca escreve de volta.

O índice pode ficar atrás das notas, e ficar atrás não impede leitura nem escrita.

O dado derivado é calculado por cofre, então tema que atravessa cofres não forma agrupamento.

## Busca

A busca só retorna nota de cofre de que o usuário é membro.

O ramo semântico depende de provedor de embedding configurado, e fica indisponível sem ele.

Uma instalação usa um modelo de embedding por vez, e trocar de modelo obriga reconstruir a
tabela de vetores.

## Autenticação

O acesso exige autenticação, por credencial local ou por provedor externo habilitado na
configuração.

Auto-registro fica desligado por padrão.

A conta tem chave estável própria, e o identificador do provedor fica guardado à parte, para
que trocar de provedor ou de email não perca o vínculo com o histórico.

## Email e notificação

O envio de email sai do caminho da requisição.

Sem provedor de email configurado, o sistema não envia e os fluxos que dependem de envio
ficam indisponíveis, inclusive a recuperação de acesso por credencial local.

## Auditoria

Toda operação crítica grava uma linha de auditoria na mesma transação da operação auditada.

A linha registra tenant, ator, ação, alvo, momento e resultado.

A aplicação não atualiza nem exclui linha de auditoria.
