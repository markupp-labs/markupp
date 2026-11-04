# ADR-0048: Notas públicas lidas sem conta

## Status

Proposta

## Contexto

Quem instala o markupp quer abrir notas para leitura de qualquer pessoa: uma nota avulsa, uma
pasta ou o cofre inteiro, como a documentação de um projeto ou uma base aberta à comunidade,
mesmo quando o resto do cofre é privado. Hoje toda rota de nota exige conta, e o RLS do
ADR-0034 recorta pelo tenant e pelo usuário da transação do ator

## Decisão

A publicação tem três alvos: uma nota, uma pasta (com as notas que entrarem nela depois) ou o
cofre inteiro, que é a pasta raiz. Publicar uma nota publica só ela. A nota pode ser publicada
por quem pode editá-la; pasta e cofre, só por owner e admin, que também despublicam qualquer
coisa do tenant. Cada ação gera evento de auditoria.

Publicar pede confirmação explícita, como tornar público um repositório no GitHub: o painel
diz o que fica público e exige digitar o nome do alvo, e a API e o MCP pedem o mesmo nome no
pedido.

Toda nota pública tem o endereço `/p/{titulo}-{id}`. O id é público e criado ao publicar; o id
interno da nota nunca sai do servidor. O título é só enfeite e redireciona quando muda.
Despublicar invalida o id, e publicar de novo gera outro.

A leitura pública roda num papel `markupp_reader`, sem login, sem superusuário e sem
`BYPASSRLS`, numa transação própria que zera tenant, usuário e papel da sessão. O papel só
recebe `SELECT` nas tabelas de nota e de organização, com política que limita às notas
publicadas. Contas, tenants, vínculos, membros, convites, auditoria e emails ficam sem grant.

Um `[[link]]` para nota não publicada vira texto puro, e o `![[embed]]` dela não aparece; nada
revela que ela existe. Busca e grafo públicos só enxergam notas publicadas. A página sai com
`noindex`, e quem publica liga a indexação por publicação. Nota não publicada e id inexistente
respondem igual, 404. Settings lista tudo que está público no tenant, e o painel marca a nota
pública

## Alternativas consideradas

- Só o cofre inteiro: obriga a separar em cofres o que se quer abrir
- Publicar a nota com as de baixo, como as subpáginas do Notion: no markupp o que agrupa é a
  pasta, publicada à parte
- Endereço por slug escolhido: disputa de nome entre tenants e quebra ao renomear
- Id da nota no endereço: expõe o id interno, e republicar devolveria os links antigos
- Ler com o papel da conexão e filtrar na aplicação: uma query sem `WHERE` vaza tudo (ADR-0016)

## Consequências

- O banco impede a leitura anônima de alcançar nota não publicada, e o papel não escreve
- Mover uma nota para fora de uma pasta publicada a torna privada
- As rotas públicas são superfície sem autenticação, e o limite por IP é a defesa contra
  raspagem e abuso de busca
