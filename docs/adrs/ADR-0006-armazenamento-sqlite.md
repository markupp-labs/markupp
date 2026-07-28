# ADR-0006: SQLite como armazenamento dos documentos e anexos

## Status

Aceita

## Contexto

Precisamos decidir onde o conteúdo markdown e os anexos embutidos ficam guardados no servidor

## Decisão

SQLite armazena o conteúdo dos documentos e dos anexos, além dos metadados. O ADR-0003 já escolheu SQLite como banco, aqui fica formalizado que ele também é a fonte de verdade do conteúdo

## Alternativas consideradas

- Filesystem no volume do servidor: exige coordenação entre disco e banco
- Garage (object storage): serviço extra no compose sem ganho para o escopo

## Consequências

- Conteúdo e metadados gravados em uma única transação
- Backup é copiar um arquivo só
- Tamanho do banco cresce conforme o volume de anexos
