# ADR-0008: AGPL v3 como licença do servidor

## Status

Aceita

## Contexto

O servidor precisa de uma licença alinhada às premissas do projeto: self-host, soberania do dado e abertura a múltiplos clientes

## Decisão

AGPL v3

## Alternativas consideradas

- MIT: permissiva, mas permite que terceiros ofereçam o servidor como SaaS fechado sem contribuir de volta
- Apache 2.0: permissiva com cláusula de patente, mesma vulnerabilidade do MIT a SaaS fechado
- GPL v3: copyleft forte, mas não cobre uso como serviço de rede
- SSPL: copyleft hostil a provedores de nuvem, não amplamente aceita como open-source

## Consequências

- Modificações distribuídas, inclusive via rede, precisam ser liberadas sob AGPL
- Adoção corporativa para uso embarcado em produto fechado fica desincentivada
- Self-host pessoal não é afetado por obrigações adicionais
