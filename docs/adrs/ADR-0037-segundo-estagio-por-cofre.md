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

Quando o cofre ainda não tinha df, na primeira vez ou depois do reindex, o segundo estágio
reenfileira as notas do cofre, para o lote reescolher os 50 termos de cada uma com o idf
calculado. O reenfileiramento não marca o cofre, e o lote que regrava as mesmas notas sem
mudar título, caminho nem ligação também não, então o ciclo acaba no lote seguinte

A reserva do cofre acaba no commit do lote. O segundo estágio roda sob uma trava consultiva
da transação, por cofre: outro Job no segundo estágio do mesmo cofre faz o seguinte pular, e
o cofre só marcado não é reservado enquanto a trava existe. Um lote pode gravar o cofre
durante o segundo estágio; a marcação dele aumenta a revisão, o segundo estágio desmarca só
a revisão que leu, e o próximo Job recalcula. Recalcular sem mudança grava o mesmo, a menos
da tolerância de convergência do PageRank

Quando o lote passa de 50 notas mais 10% das notas do cofre, o Job roda ANALYZE das tabelas
derivadas que o lote grava antes do segundo estágio. É o limiar padrão do autovacuum: o
segundo estágio roda logo depois do commit, antes de o autovacuum acordar, e sem a
estatística da carga nova o planejador trata as tabelas como se tivessem o tamanho anterior.
O primeiro lote de um cofre importado é o caso típico; o lote incremental não passa do
limiar

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

## Consequências

- Edição que não muda ligação nem título não paga o custo do cofre inteiro
- O cofre novo e o cofre reindexado passam pelo lote duas vezes: a primeira escolhe os
  termos só pelo tf, a segunda com o idf do cofre
- O df fica levemente defasado entre uma mudança de ligação ou título e outra; o lote pesa
  as postagens das notas dele com o df e o N vigentes
- Lote e segundo estágio do mesmo cofre podem rodar ao mesmo tempo em Jobs diferentes, e a
  edição não espera o cálculo do cofre inteiro. A repesagem das postagens vem por último e
  pula a nota que um lote concorrente está regravando, que fica com o df anterior
- Quem precisar de mais dado do cofre inteiro acrescenta um passo ao segundo estágio, sem
  novo disparo
