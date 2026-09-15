# ADR-0031: Indexadores em Go

## Status

Aceita

## Contexto

O ADR-0002 dividiu o sistema em processos e escolheu a linguagem por serviço, deixando
indexação e busca semântica em Rust ou Python. A decisão é de antes do MVP, quando o
indexador era uma ideia sem forma.

Desde então, o ADR-0028 descreveu os dois indexadores como serviços do mesmo conjunto de
imagens, e o ADR-0029 colocou o modelo local dentro do processo do indexador de embedding,
via ONNX Runtime com cgo. Os dois falam de serviços Go sem dizer isso, e nenhum substitui o
ADR-0002 nesse ponto

## Decisão

Os indexadores determinístico e de embedding são escritos em Go, como os demais serviços. A
divisão em processos independentes do ADR-0002 continua valendo; o que sai é a escolha de
Rust ou Python para eles

## Alternativas consideradas

- Manter a escolha em aberto: preserva a liberdade do ADR-0002 e deixa a decisão sem dono,
  com dois ADRs aceitos descrevendo o mesmo serviço em linguagens diferentes
- Indexação em Python: o ecossistema de modelos é mais rico e dispensa cgo para carregar
  pesos, ao custo de uma segunda toolchain no CI e de um segundo acesso ao banco escrito
  fora do storage que o ADR-0010 isola

## Consequências

- Substitui o ADR-0002 na escolha de linguagem para indexação e busca semântica, e mantém o
  resto dele intacto
- O código gerado pelo `sqlc` e a camada de storage do ADR-0010 servem também aos
  indexadores, sem uma segunda tradução de schema
- O CI segue rodando só Go
- O indexador de embedding carrega ONNX Runtime via cgo, então é o único serviço sem build
  puramente Go (ADR-0029)
- Trocar o modelo de embedding por um que só exista em Python passa a custar um serviço em
  outra linguagem, e não mais uma troca de biblioteca
