# Markupp

Servidor de anotações em markdown que organiza a base sozinho. O usuário escreve e salva, e
o servidor indexa, liga e agrupa sem alterar o conteúdo. Toda interação passa por uma API
REST versionada, e painel web, plugin de editor e agente de IA são clientes pares dela.

Descrição completa em [resumo do projeto](docs/resumo.md).

## Equipe:
 - Renato Freitas - Arquiteto de Software;
 - Nícolas Arthur - DevOps/Infra;
 - Nicolas Pitz - Engenheiro de Qualidade;
 - Gabriela Riedel - Scrum Master;

## MVP
### O problema
Excesso de informação descentralizada e dados desestruturados sem métodos de busca de conteúdo. Nossa solução tem o objetivo de resolver esta dor.

### Público Alvo
- Usuários que já têm agentes de IA integrados no seu dia a dia.
- Times que precisam centralizar documentos. 

### O que fará
- Prover um ambiente centralizado para criação, edição e organização de documentos Markdown.
- Estruturar o conhecimento em hierarquia lógica.
- Expor uma API REST que permita integração com clientes
- Processar documentos automaticamente para busca semântica

## MVP entregue (Release Candidate)

As seções acima são a visão do produto. O MVP entregue cobre CRUD de notas pela
API REST, listagem, busca por substring e sincronização com detecção de conflito
e sobrescrita forçada.

Fora do MVP: versionamento, organização hierárquica, busca semântica e
autenticação. Ver [requisitos do MVP](docs/requisitos-mvp.md) e
[limitações conhecidas](docs/limitacoes-conhecidas.md).

> Para subir o servidor e validar o ambiente, veja [docs/DEPLOY.md](docs/DEPLOY.md).

## Governança
**Q: Quem pode abrir PR?**
> `A: Todos.`

**Q: Quem pode aprovar PR?** 
> `A: Um par.`

**Q: Politica da main**
> `A: Só aceita merge com PR aprovado por par.`

**Q: Commits**
> `A: Seguem o padrão conventional commits em português.`


## DoD
- Cobertura de testes unitários mínima de 80%
- Feature revisada e aprovada por pares

## Critérios de Qualidade
- [Critérios de Qualidade](docs/qualidade.md)

## Fluxo de trabalho
O fluxo de trabalho do projeto está documentado em [fluxo de trabalho](docs/fluxo-de-trabalho.md)

## Arquitetura
- [ADRs](docs/adrs/)
