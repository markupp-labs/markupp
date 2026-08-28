# ADR-0019: Embedding com provedor configurável

## Status

Aceita

## Contexto

Busca semântica e sugestão de ligação precisam de vetor por nota ou por trecho. As duas
edições têm restrições opostas: o Enterprise roda indexação em Lambda (ADR-0018), onde
carregar pesos de modelo significa imagem grande e cold start, e o self-host não deve depender
de serviço de terceiro para funcionar

## Decisão

A geração de embedding fica atrás de uma interface com provedor escolhido por configuração.
Duas implementações: cliente HTTP compatível com a API da OpenAI, e cliente do Amazon Bedrock.
O padrão do Enterprise é Bedrock. No self-host, o cliente compatível com OpenAI aponta para
Ollama, LocalAI, llama.cpp ou vLLM no próprio compose, e busca semântica vem desligada até que
um provedor seja configurado

## Alternativas consideradas

- Só Bedrock: obriga o self-host a ter credencial da AWS, contrariando o objetivo de rodar sem
  serviço de terceiro
- Modelo carregado dentro do processo de indexação, em Go: exigiria ONNX Runtime via cgo, o
  que derruba a build puramente Go, e inviabiliza o Lambda
- Serviço próprio de embedding em Rust: mais um deployable e mais uma linguagem para manter,
  para entregar o que um Ollama no compose já entrega pelo mesmo endpoint compatível
- Provedor local como terceira implementação: desnecessário, porque toda ferramenta local de
  inferência expõe endpoint compatível com a API da OpenAI

## Consequências

- Cada edição usa o provedor que lhe serve, com uma interface e duas implementações
- Instalação padrão do self-host não fala com serviço externo, e não tem busca semântica até
  ser configurada. Busca léxica (ADR-0017) e grafo de ligações funcionam sem vetor
- Vetores de provedores diferentes não são comparáveis, e a dimensão varia por modelo: cada
  instância tem um modelo ativo, registrado junto dos vetores, e trocar de modelo obriga a
  reconstruir a tabela de vetores inteira
- Reconstruir é seguro porque vetor é dado derivado (ADR-0012), mas custa uma passada de
  embedding sobre todas as notas, o que no Enterprise é custo por chamada
- O custo do Bedrock por nota e por reindexação completa ainda precisa ser medido numa prova de
  conceito no free tier da AWS. Custo acima do previsto troca o provedor padrão do Enterprise,
  sem mudança de arquitetura
