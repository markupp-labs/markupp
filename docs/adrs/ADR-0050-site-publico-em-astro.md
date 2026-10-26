# ADR-0050: Site público em Astro, em repositório próprio

## Status

Proposta

## Contexto

O markupp precisa de um site público com apresentação do produto e documentação, com
animações e transições no nível dos sites de produto de referência. A documentação deve ser
escrita no próprio markupp, num cofre público (ADR-0048), e ter a mesma aparência das notas. O
site é da equipe: não faz parte de nenhuma edição e não vai para quem instala o self-host

## Decisão

O site é um projeto Astro num repositório privado da organização, fora deste repositório e do
componente de deploy reutilizável. As páginas de marketing são pré-renderizadas, sem JavaScript
por padrão, e as animações vêm de CSS com animações ligadas à rolagem, View Transitions e a
biblioteca Motion só nas ilhas que precisam. Não há framework de interface.

As páginas de documentação são renderizadas sob demanda pelo adaptador Node, lendo a API
pública do cofre com `If-None-Match` e guardando em memória pela versão do grafo. Editar uma
nota no cofre atualiza o site sem novo build.

Para que nota e documentação não divirjam, o renderizador de markdown do painel sai para um
pacote npm publicado por este repositório, consumido pelo painel e pelo site

## Alternativas consideradas

- Next.js: animação no mesmo nível, mas traz React, um terceiro paradigma de interface para a
  equipe, e a revalidação incremental fora da Vercel pede cache próprio no Kubernetes
- Páginas dentro do painel Rails: entregaria marketing a todo self-host, prenderia o site ao
  ciclo de release do painel, e o importmap sem bundler limita as animações
- Aplicação Rails separada: mesma habilidade da equipe, mas pesada para um site quase
  estático e exigiria um renderizador de markdown em Ruby, que divergiria do markdown-it
- Build estático com rebuild agendado: mais simples de servir, mas a documentação atrasa em
  relação ao cofre

## Consequências

- O site e o deploy dele ficam num repositório privado da equipe, fora deste repositório e do
  componente de deploy reutilizável
- O renderizador passa a ter versão publicada, e mudar a renderização exige publicar o pacote
- O site depende da disponibilidade da API pública para servir a documentação, e o cache em
  memória cobre quedas curtas
