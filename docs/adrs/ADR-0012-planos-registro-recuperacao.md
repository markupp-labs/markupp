# ADR-0012: Plano de registro separado do plano de recuperação

## Status

Aceita

## Contexto

A busca vive hoje dentro do repositório de notas (`SearchNotes` monta um `content LIKE`),
misturando duas responsabilidades com requisitos opostos: guardar o que os clientes
enviaram e otimizar a recuperação desse conteúdo. A primeira precisa ser durável e ter
schema estável, a segunda precisa ser descartável e trocar de algoritmo com frequência

## Decisão

Dois planos. O plano de registro guarda as notas e é a fonte da verdade. O plano de
recuperação é derivado do registro, reconstruível a qualquer momento e nunca escreve de
volta. Índices, grafo de ligações e agrupamentos vivem no plano de recuperação

A separação é lógica e não decide topologia de processo: o ADR-0002 segue valendo para isso

## Alternativas consideradas

- Manter tudo no repositório de notas (status quo): o schema de busca fica preso ao schema
  do dado durável e trocar de algoritmo exige migration

## Consequências

- Apagar e reconstruir o índice inteiro é operação segura
- O algoritmo de recuperação muda sem migration no dado durável
- O índice pode ficar atrás do registro: a consistência é eventual por escolha
- Recuperação degradada não impede escrita e leitura de notas
- A direção é única: o plano de recuperação observa o registro, o registro não conhece o
  plano de recuperação
- A fronteira entre os planos é a costura por onde um processo separado de indexação pode
  ser extraído depois, sem mudar o modelo de dados
