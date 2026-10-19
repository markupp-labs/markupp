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
cofre, na mesma transação, depois de aplicar o lote, e só quando ligações ou títulos do cofre
mudaram: o lote que cria, renomeia ou muda as ligações de uma nota marca o cofre, e um trigger
de exclusão em notes também marca. O scaler do KEDA conta o cofre marcado como pendente, e o
reindex marca os cofres que reenfileira. Hoje o segundo estágio recalcula o df do cofre

## Alternativas consideradas

- df incremental no lote: exato e barato, mas a exclusão de nota apaga o índice em cascata
  sem passar pelo indexador, e o df nunca seria decrementado
- df na leitura, por ts_stat: sem estado, mas toda sugestão varreria o índice do cofre
- Recalcular a cada lote: simples, mas cada edição pagaria o custo do cofre inteiro
- Job ou CronJob separado para o segundo estágio: outro disparo, outra credencial e outra
  corrida com o indexador do mesmo cofre

## Consequências

- Edição que não muda ligação nem título não paga o custo do cofre inteiro
- O df fica levemente defasado entre uma mudança de ligação ou título e outra
- O cofre continua travado por um Job só, então lote e segundo estágio não disputam o cofre
- Quem precisar de mais dado do cofre inteiro acrescenta um passo ao segundo estágio, sem
  novo disparo
