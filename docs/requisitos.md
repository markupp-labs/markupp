# Requisitos

O sistema deve:

- Criar, visualizar, editar, renomear e excluir notas em markdown
- Guardar as notas de forma centralizada, com o servidor como única fonte da verdade
- Dar a cada nota um caminho lógico único dentro do cofre a que ela pertence
- Nunca alterar o conteúdo nem o caminho de uma nota por conta própria
- Calcular ligações entre notas, ligações automáticas, agrupamentos por tema e índices sem
  pedir nada ao usuário
- Deixar o usuário recusar a ligação automática entre duas notas
- Reconstruir qualquer estrutura derivada a partir das notas, a qualquer momento
- Indexar o conteúdo para busca léxica, com relevância e trecho destacado no resultado
- Gerar vetores das notas para a ligação por sentido, com provedor de embedding plugável
- Atualizar os índices automaticamente a cada edição
- Deixar o cliente navegar pela base, pela visão derivada ou pelo caminho lógico
- Publicar nota, pasta ou cofre para leitura sem conta, com indexação por buscadores opcional
- Expor uma API REST como interface principal, versionada por prefixo de caminho
- Documentar essa API num contrato OpenAPI
- Aceitar vários clientes como pares da mesma API, sem cliente privilegiado
- Servir um painel web na mesma origem da API, utilizável em tela de celular e de desktop
- Atender agentes de IA por um servidor MCP, cliente da API REST
- Autorizar clientes de terceiros por OAuth, com escopos e revogação pela conta
- Controlar o acesso por autenticação, com email e senha e provedores de login externo
- Abrir e fechar o cadastro por método
- Tratar o cofre como unidade de acesso, onde quem é membro lê e escreve todas as notas dele
- Separar os dados de cada tenant dentro do banco, sem depender de filtro escrito em cada
  consulta
- Persistir contas, tenants, cofres, notas e dados derivados no PostgreSQL
- Enviar email por SMTP, fora do caminho da requisição, e rodar sem email quando não houver
- Registrar operações críticas numa trilha de auditoria que só aceita inserção
- Subir a instância inteira com `docker compose up`, a partir das imagens publicadas
- Instalar em Kubernetes por um componente Pulumi
- Expor uma única porta, com API, painel e MCP sob a mesma URL pública
- Gerar na primeira subida os segredos internos do servidor e guardá-los no banco
- Configurar-se por variável de ambiente e, no que vale para a instância, pelo painel, com a
  variável de quem hospeda vencendo
- Emitir log estruturado em JSON
