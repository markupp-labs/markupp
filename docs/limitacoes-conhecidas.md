# Limitações Conhecidas (Release Candidate)

- Sem versionamento/histórico: a sincronização detecta conflito (`409`) e
  sobrescreve, mas não guarda versões
- Sem autenticação: uso local/self-hosted (ADR-0005), não exponha fora de
  `localhost`
- Sem organização hierárquica: notas por `path`, sem árvore de pastas
- Sem busca semântica: busca por substring no conteúdo
- Validação ponta a ponta pelo cliente é manual: o servidor tem testes de
  integração, o percurso do usuário não
- Imagem Docker: prefira tag versionada à `latest` ([DEPLOY](DEPLOY.md))

Itens fora do escopo do MVP: [requisitos do MVP](requisitos-mvp.md).
