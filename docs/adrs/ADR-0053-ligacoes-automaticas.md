# ADR-0053: Ligações automáticas

## Status

Proposta

## Contexto

A premissa diz que o servidor liga as notas. Hoje ele só deriva a ligação inferida, que nasce
de uma ligação escrita que não resolve (ADR-0039). Quem não escreve a ligação fica com a nota
órfã ou fora da comunidade certa, mesmo citando o título de outra ou tratando do mesmo assunto.
A menção não ligada e a sugestão já acham esses pares, mas só para quem lê

## Decisão

O servidor cria a ligação automática por menção, por texto parecido e por sentido. Ela vive só
no plano de recuperação, numa tabela própria com tipo, motivo e pontuação, e entra em tudo que
lê o grafo. Entre duas notas há no máximo uma, escolhida nessa ordem, e nenhuma quando as duas
já têm ligação escrita ou inferida, em qualquer sentido

- Por menção: a nota que cita o título de outra no texto liga a todas as notas com o título,
  com o mesmo filtro de título curto, stopword e comum da menção não ligada (ADR-0037). O lote
  refaz as menções das notas que mudou, e o segundo estágio refaz as do cofre
- Por texto parecido: cada nota entre as 5 de maior cosseno tf-idf da outra, kNN mútuo (von
  Luxburg, 2007), com cosseno mínimo de 0,2, gravada nos dois sentidos e calculada no lote
- Por sentido: o mesmo kNN mútuo sobre os vetores do indexador de embedding, sem mínimo, que
  muda de modelo para modelo. Só existe quando a instalação tem provedor de embedding

A vizinhança no grafo não cria ligação: cada ligação nova traria vizinhos novos. No PageRank e
no Louvain, que passam a ser ponderados, a ligação automática pesa 0,5 e a escrita e a inferida
pesam 1. Os números não vêm de avaliação (ADR-0038)

A recusa de ligação fica no plano de registro, por cofre. Ela vale para o par nos dois sentidos
e para qualquer tipo, até ser desfeita, e o servidor não cria ligação automática entre o par
recusado

## Alternativas consideradas

- Manter menção e sugestão só como lista: nada muda no grafo, e a nota continua órfã
- Peso igual ao da escrita, como a inferida: um palpite sem escrita nenhuma pesaria tanto
  quanto o que o usuário escreveu
- Corte por cosseno fixo ou pelas k primeiras de cada nota: o primeiro depende do cofre, o
  segundo deixa a nota genérica virar centro de tudo
- Uma ligação por tipo entre o mesmo par: o par citado e parecido somaria pesos
- Recusa por tipo: recusada a menção, o texto parecido ligaria o mesmo par no lugar
- Gravar a ligação na nota: altera o conteúdo (ADR-0013)

## Consequências

- A nota que cita ou se parece com outra deixa de ser órfã e entra na comunidade dela
- O palpite errado também entra nas medidas, com metade do peso, e a taxa de erro não é medida
- A menção não ligada e a sugestão deixam de mostrar o par que já tem ligação automática
- A recusa é o único dado da organização derivada que vive no plano de registro
- A inferida do ADR-0039 continua pesando como a escrita
