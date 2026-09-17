# Requisitos

O sistema deve:

- Criar, visualizar, editar, renomear e excluir notas em markdown
- Guardar as notas de forma centralizada, com o servidor como única fonte da verdade
- Dar a cada nota um caminho lógico único dentro do cofre a que ela pertence
- Nunca alterar o conteúdo nem o caminho de uma nota por conta própria
- Calcular ligações entre notas, agrupamentos por tema e índices sem pedir nada ao usuário
- Reconstruir qualquer estrutura derivada a partir das notas, a qualquer momento
- Indexar o conteúdo para busca léxica, com relevância e trecho destacado no resultado
- Indexar o conteúdo em vetores para busca semântica
- Atualizar os índices automaticamente a cada edição
- Buscar por título e por conteúdo, combinando o ramo léxico e o semântico
- Deixar o cliente navegar pela base, pela visão derivada ou pelo caminho lógico
- Expor uma API REST como única interface, versionada por prefixo de caminho
- Publicar a documentação dessa API junto com o servidor
- Aceitar vários clientes como pares da mesma API, sem cliente privilegiado
- Servir um painel web ao navegador, com o código baixado sob demanda
- Atender agentes de IA por um servidor MCP que expõe o mesmo conjunto de operações da
  API REST, isolado dela por perfil de carga
- Controlar o acesso por autenticação, com credencial local e provedores externos
  habilitados por configuração
- Tratar o cofre como unidade de acesso, onde quem é membro lê e escreve todas as notas dele
- Separar os dados de cada tenant dentro do banco, sem depender de filtro escrito em cada
  consulta
- Persistir usuários, cofres, notas e dados derivados em banco de dados
- Enviar email e notificações por provedor configurável, fora do caminho da requisição
- Registrar toda operação crítica numa trilha de auditoria que só aceita inserção
- Publicar a documentação da modelagem de dados e da arquitetura
- Rodar em cenário de desenvolvimento e em cenário de produção, com a mesma imagem de
  container
- Provisionar a infraestrutura de nuvem por IaC
- Implantar em produção automaticamente por CI/CD
- Terminar TLS em toda porta exposta
- Responder com baixa latência sob carga de leitura
- Manter o painel web utilizável em tela de celular e de desktop
- Operar com custo mínimo
- Configurar-se por variável de ambiente, sem arquivo montado no contêiner
- Não guardar estado no disco do contêiner
- Emitir log estruturado, métrica e rastro para coleta externa
- Rodar self-hosted numa instalação de um nó, com o mesmo código que roda na nuvem
