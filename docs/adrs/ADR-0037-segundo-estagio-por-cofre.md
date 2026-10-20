# ADR-0037: Segundo estágio do indexador por cofre

## Status

Aceita

## Contexto

O ADR-0033 faz do indexador uma função do conjunto de mudanças. Parte do plano de
recuperação depende do cofre inteiro: a frequência de documento do idf, as ligações
inferidas, o PageRank e as comunidades. Uma nota nova ou apagada muda esses valores para
notas que não estão no lote

## Decisão

O indexador ganha um segundo estágio, a única exceção ao ADR-0033. Ele roda no mesmo Job do
cofre, depois do commit do lote, em transação própria, e só quando ligações ou títulos do
cofre mudaram: o lote que cria, renomeia, move de pasta ou muda as ligações de uma nota
marca o cofre, e um trigger de exclusão em notes também marca. O scaler do KEDA conta o
cofre marcado como pendente, e o reindex marca os cofres que reenfileira e descarta o df
deles. Hoje o segundo estágio recalcula as
ligações inferidas, o PageRank, as comunidades, o df e o N do cofre, e repesa com eles as
postagens tf-idf e a norma de cada nota

Quando o cofre ganha df, na primeira vez, depois do reindex ou depois de um df de N zero, o
segundo estágio reenfileira as notas do cofre, para o lote reescolher os 50 termos de cada
uma com o idf calculado. O df de N zero não reenfileira: nenhuma nota tem documento nem
termo para reescolher, e a que ainda não tem documento já está na fila. O df e o N saem com
o cofre, em cascata. O reenfileiramento não marca o cofre, e o lote que regrava as mesmas notas sem
mudar título, caminho nem ligação também não, então o ciclo acaba no lote seguinte

A reserva do cofre acaba no commit do lote. O segundo estágio roda sob uma trava consultiva
da transação, por cofre: outro Job no segundo estágio do mesmo cofre faz o seguinte pular, e
o cofre só marcado não é reservado enquanto a trava existe. Um lote pode gravar o cofre
durante o segundo estágio; a marcação dele aumenta a revisão, o segundo estágio desmarca só
a revisão que leu, e o próximo Job recalcula. Recalcular sem mudança grava o mesmo, a menos
da tolerância de convergência do PageRank

A revisão lida vira a versão do grafo do cofre, gravada na mesma transação do que o segundo
estágio calcula. As leituras do grafo para desenhar usam essa versão como validador de cache

Quando o lote passa de 50 notas mais 10% das notas do cofre no último df, o Job roda ANALYZE
das tabelas derivadas que o lote grava antes do segundo estágio. O N do df evita contar as
notas do cofre a cada lote. É o limiar padrão do autovacuum: o
segundo estágio roda logo depois do commit, antes de o autovacuum acordar, e sem a
estatística da carga nova o planejador trata as tabelas como se tivessem o tamanho anterior.
O primeiro lote de um cofre importado é o caso típico; o lote incremental não passa do
limiar

O PageRank para quando a norma 1 da diferença entre duas iterações fica abaixo de N × 1e-6,
o critério usual, ou em 100 iterações, e o Louvain em 10 níveis e 100 passadas
de movimentos locais por nível. O cálculo que para no teto grava o resultado que tem e vira
um aviso no log JSON do Job, com o algoritmo, o cofre, as iterações (níveis, no Louvain) e as
passadas. O PageRank é uma iteração de potência própria, a partir do vetor uniforme, porque
o do gonum não aceita teto e parte de um vetor aleatório; um teste o confere contra o do
gonum

## Alternativas consideradas

- df incremental no lote: exato e barato, mas a exclusão de nota apaga o índice em cascata
  sem passar pelo indexador, e o df nunca seria decrementado
- df na leitura, por ts_stat: sem estado, mas toda sugestão varreria o índice do cofre
- Recalcular a cada lote: simples, mas cada edição pagaria o custo do cofre inteiro
- Job ou CronJob separado para o segundo estágio: outro disparo, outra credencial e outra
  corrida com o indexador do mesmo cofre
- Segundo estágio na transação do lote: um snapshot só, mas a reserva do cofre fica presa
  durante o cálculo do cofre inteiro e o lote seguinte espera por ele
- ANALYZE a cada lote, ou nunca: o primeiro paga a amostragem em toda edição, o segundo
  deixa o primeiro segundo estágio de um cofre importado planejar sem estatística
- PageRank e Louvain sem teto: um grafo patológico prenderia o Job sem aviso

## Consequências

- Edição que não muda ligação nem título não paga o custo do cofre inteiro
- O cofre novo e o cofre reindexado passam pelo lote duas vezes: a primeira escolhe os
  termos só pelo tf, a segunda com o idf do cofre
- O df fica levemente defasado entre uma mudança de ligação ou título e outra; o lote pesa
  as postagens das notas dele com o df e o N vigentes
- Lote e segundo estágio do mesmo cofre podem rodar ao mesmo tempo em Jobs diferentes, e a
  edição não espera o cálculo do cofre inteiro. A repesagem das postagens vem por último e
  pula a nota que um lote concorrente está regravando, que fica com o df anterior
- A versão do grafo só muda no segundo estágio: entre o commit de um lote que muda ligações
  e o segundo estágio seguinte, o cliente que revalida recebe 304 com o grafo anterior
- A diferença em norma 1 cai a 0,85 por iteração a partir de no máximo 2, então o PageRank
  converge em menos de 100 iterações em qualquer cofre; o teto só segura erro de ponto
  flutuante. O erro que sobra fica abaixo de N × 1e-6 × 0,85/0,15 na norma 1, abaixo da
  precisão que a ordenação por PageRank usa
- Quem precisar de mais dado do cofre inteiro acrescenta um passo ao segundo estágio, sem
  novo disparo
