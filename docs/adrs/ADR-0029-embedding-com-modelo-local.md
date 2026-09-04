# ADR-0029: Embedding com provedor configurável, incluindo modelo local

## Status

Aceita

## Contexto

O ADR-0019 descartou o modelo local porque a indexação rodava em Lambda, onde carregar pesos
significa imagem grande e cold start, e concorrência provisionada para contornar eliminaria a
economia. Com a indexação em Job no Kubernetes (ADR-0027), essa restrição deixa de existir: o
Job sobe, carrega o modelo, processa o lote e termina

## Decisão

Três implementações atrás da mesma interface: cliente HTTP compatível com a API da OpenAI,
cliente do Amazon Bedrock, e modelo local carregado no processo do indexador de embedding. O
Bedrock segue como padrão do Enterprise, e o modelo local passa a ser a opção de self-host sem
depender de serviço de terceiro

## Alternativas consideradas

- Manter apenas os dois provedores do ADR-0019: quem quer local aponta para um Ollama no próprio
  cluster e resolve sem código novo, ao custo de mais um componente para o self-hoster subir e
  manter
- Modelo local como padrão em todas as edições: elimina a dependência de terceiro por completo,
  e obriga baixar pesos na primeira execução, inclusive onde o Bedrock sairia mais barato

## Consequências

- Substitui o ADR-0019
- Busca semântica no self-host deixa de exigir serviço externo, resolvendo o conflito que o
  ADR-0019 registrava com o objetivo de não depender de terceiro
- O indexador de embedding passa a ter imagem grande, o que só é aceitável porque ele é serviço
  separado (ADR-0028) e não pesa nos demais
- Carregar modelo em Go exige ONNX Runtime via cgo, o que derruba a build puramente Go nesse
  serviço, e apenas nele
- Vetor de provedores diferentes continua incomparável: um modelo ativo por instalação, e
  trocar obriga reconstruir a tabela de vetores inteira
