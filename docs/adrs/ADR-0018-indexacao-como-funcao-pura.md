# ADR-0018: Indexação como função pura, com invocação por edição

## Status

Aceita

## Contexto

Indexação, montagem do grafo e agrupamento são trabalho de rajada e uso intenso de CPU,
enquanto a API é constante e leve. Manter uma máquina dimensionada para o pico ligada o tempo
todo desperdiça recurso no Enterprise. Ao mesmo tempo, o self-host não deve precisar de fila
nem de função em nuvem para funcionar

## Decisão

O indexador é escrito como função de (tenant, conjunto de mudanças) para linhas derivadas,
sem estado próprio e sem saber quem o invocou. Dois entrypoints finos sobre o mesmo pacote:
goroutine dentro do binário no self-host, e handler Lambda no Enterprise. O conjunto de
mudanças vem do cursor de revisão usado pela sincronização incremental

## Alternativas consideradas

- Indexar dentro do caminho da requisição: simples, mas coloca trabalho de rajada na latência
  de escrita
- Serviço de indexação sempre ligado nas duas edições: recurso ocioso no Enterprise e um
  processo a mais para o self-host operar
- Broker de mensagens desde o início (SQS, Kafka): consultar o cursor de revisão resolve sem
  serviço novo, e broker entra quando o custo do polling for medido e doer

## Consequências

- Uma implementação de indexação, duas formas de invocar, sem edição com funcionalidade a
  menos
- Lambda com PostgreSQL exige pool de conexão na frente, com RDS Proxy ou pgbouncer, e isso é
  custo fixo do Enterprise
- O trabalho precisa ser fatiado por lote, porque Lambda tem teto de quinze minutos
- Indexação atrasada não impede leitura nem escrita de nota, o que cumpre fisicamente o que o
  ADR-0012 prometeu apenas no desenho
- O self-host não ganha dependência de nuvem
