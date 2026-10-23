# ADR-0024: Cofre como namespace com lista de acesso

## Status

Aceita

## Contexto

Dentro de um tenant, nem toda nota deve ser visível para todo mundo. É preciso separar o que é
do time do que é de cada pessoa, sem transformar isso em regra por nota, que é cara de
administrar e fácil de errar

## Decisão

O cofre é a unidade de namespace e de acesso. A nota pertence a exatamente um cofre, e o acesso
à nota é consequência de ser membro do cofre. Membro é binário: quem é membro lê e escreve,
salvo o viewer do tenant, que pode ser membro e só lê. Quem não é viewer cria, renomeia e apaga
cofre, e adiciona e remove membro. O membro adicionado é uma conta que já tem vínculo com o
tenant; quem é de fora entra antes pelo convite do tenant. Qualquer membro, viewer inclusive,
sai do cofre. Todo usuário recebe um cofre na criação, que é um cofre comum. O nome do cofre
não é único. O `path` é único por cofre, e ligação não atravessa cofre

## Alternativas consideradas

- Visibilidade como atributo da nota: o acesso vira regra por nota, mais granular e muito mais
  caro de administrar e de auditar
- Papéis de leitura, escrita e dono: cobriria cofre de referência que muitos leem e poucos
  escrevem, ao custo de um sistema de permissão que ainda não é necessário
- `path` único por tenant, com o cofre como coluna: manteria a auto-organização atravessando
  cofres, ao custo de o cofre deixar de ser namespace e de precisar de filtro por acesso em
  cada consulta derivada

## Consequências

- O dado derivado é calculado por cofre: índice, grafo, agrupamento e vetores nascem por cofre
- A auto-organização acontece dentro do cofre, não na base inteira, então tema que atravessa
  cofres não forma agrupamento
- Não existe vazamento em agregado: rótulo e resumo derivam de um cofre e são vistos por quem é
  membro dele
- A política de RLS ganha uma segunda camada, sobre pertencimento ao cofre, além da separação
  por tenant do ADR-0016
- Mover nota entre cofres é apagar na origem e criar no destino, então a nota recebe id novo e
  não leva histórico junto
- Qualquer membro remove qualquer membro, inclusive quem criou o cofre, e um cofre pode acabar
  sem nenhum membro
- O cofre criado junto com o usuário é comum, então "pessoal" é convenção de uso e não garantia
  do sistema
- Nome repetido não revela que existe cofre de que a conta não é membro, ao custo de dois
  cofres poderem aparecer com o mesmo nome
- Apagar o cofre apaga as notas dele e o dado derivado delas, e o último cofre da conta também
  pode ser apagado ou abandonado
