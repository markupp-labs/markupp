# ADR-0032: Painel web em Ruby on Rails, cliente da API

## Status

Aceita

## Contexto

O ADR-0028 previu o painel web como arquivo estático em armazenamento de objeto com CDN, sem
processo próprio. Nenhum painel foi escrito, e a disciplina exige frontend no navegador, então
a escolha do que construir seguia aberta

## Decisão

O painel web é uma aplicação Ruby on Rails com Hotwire e Turbo, mantida neste repositório, e
consome o servidor Go pela API REST versionada como qualquer outro cliente. Não alcança o
PostgreSQL

## Alternativas consideradas

- Página única em React, compilada para estático e servida por CDN: é o que o ADR-0028 previa,
  sem processo, sem custo de computação e sem segunda toolchain, ao custo de escrever a
  interface inteira em JavaScript
- Rails com ActiveRecord direto no banco: é o Rails idiomático e dispensa o salto de rede, e
  duplicaria tenant, cofre e auditoria em Ruby sobre o mesmo schema, contra o ADR-0007 e o
  ADR-0026

## Consequências

- Substitui o item de painel web do ADR-0028. O painel passa a consumir computação, com
  processo, container e escala próprios, e deixa de custar só armazenamento e transferência
- O CI daqui volta a rodar duas toolchains, Go e Ruby, revertendo o ganho que o ADR-0030
  registrou ao tirar o plugin do repositório
- Autenticação, isolamento por tenant e auditoria continuam existindo em um lugar só, porque o
  painel não alcança o banco
- Cada interação do usuário custa um salto de rede a mais, do painel para a API
- Hotwire manda HTML pelo fio, e o requisito da disciplina fala em código descarregado e
  executado no navegador. Se a avaliação ler ao pé da letra, o painel precisa passar a entregar
  um pacote de JavaScript
- O painel é mais um cliente sobre o contrato do ADR-0026, então quebrar a API aqui aparece no
  build do painel e não no do servidor
