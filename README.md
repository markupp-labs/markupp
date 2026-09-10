# Markupp
Markupp é um projeto que propõe uma arquitetura centralizada e self-hosted que permite que usuários criem, editem, organizem e versionem documentos markdown em uma estrutura hierárquica por projetos e categorias. 

O sistema deve funcionar como um hub central de conhecimento oferecendo operações completas de criação, leitura, atualização, exclusão e controle de versões, além de manter organização por caminhos lógicos semelhantes a diretórios.

Com a ascensão dos agentes de IA, a solução ideal é um repositório centralizado e multiplataforma, onde usuário e IA colaboram sobre a mesma base de dados em tempo real criando uma simbiose das suas ideias com seus agentes.

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

As seções acima são a visão do produto. O MVP entregue cobre CRUD de notas via
plugin e API REST, listagem, busca por substring e sincronização com detecção de
conflito e sobrescrita forçada.

Fora do MVP: versionamento, organização hierárquica, busca semântica e
autenticação. Ver [requisitos do MVP](docs/requisitos-mvp.md) e
[limitações conhecidas](docs/limitacoes-conhecidas.md).

## Plugin do Obsidian
O plugin vive em repositório próprio: [markupp-labs/obsidian-markupp-plugin](https://github.com/markupp-labs/obsidian-markupp-plugin).
As instruções de instalação e os artefatos das releases estão lá.

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
- [Testes de Aceitação](docs/testes-aceitacao.md)

## Fluxo de trabalho
O fluxo de trabalho do projeto está documentado em [fluxo de trabalho](docs/fluxo-de-trabalho.md)

## Arquitetura
- [Arquitetura C4](docs/arquitetura-c4.md)
- [ADRs](docs/adrs/)
