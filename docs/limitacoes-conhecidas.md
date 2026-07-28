# Limitações Conhecidas (Release Candidate)

- Sem versionamento/histórico: a sincronização detecta conflito (`409`) e
  sobrescreve, mas não guarda versões
- Sem autenticação: uso local/self-hosted (ADR-0005), não exponha fora de
  `localhost`
- Sem organização hierárquica: notas por `path`, sem árvore de pastas
- Sem busca semântica: busca por substring no conteúdo
- Validação ponta a ponta no Obsidian é manual ([testes de aceitação](testes-aceitacao.md));
  a lógica do plugin é testada
- Imagem Docker: prefira tag versionada à `latest` ([DEPLOY](DEPLOY.md))

Itens fora do escopo do MVP: [requisitos do MVP](requisitos-mvp.md).
