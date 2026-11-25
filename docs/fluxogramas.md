# Fluxogramas

Quatro fluxos, escolhidos porque cada um atravessa uma fronteira que os outros documentos
descrevem parada: a separação entre plano de registro e plano de recuperação, a troca com o
provedor de login externo, o recorte da busca pelo banco e a entrada num cofre, que depende do
vínculo com o tenant.

## Indexação de nota

```mermaid
flowchart TD
  A(["Cliente envia a nota"]) --> B["API REST valida caminho e conteúdo"]
  B --> C{"Caminho livre no cofre?"}
  C -- não --> D(["Erro de caminho ocupado"])
  C -- sim --> E["Grava a nota e a põe nas duas filas, na mesma transação"]
  E --> G(["Resposta ao cliente"])
  E --> H["Indexador determinístico pega o cofre da fila"]
  H --> I["Refaz o índice léxico e as ligações das notas que mudaram"]
  I -- "mudaram títulos ou ligações" --> P["Segundo estágio refaz grafo, comunidades e ligações por sentido do cofre"]
  E --> J{"Provedor de embedding configurado?"}
  J -- não --> K(["Nota fica sem vetor"])
  J -- sim --> L["Indexador de embedding gera o vetor da nota"]
  L --> M["Grava o vetor no PostgreSQL e marca o cofre para o segundo estágio"]
  M --> P
  P --> N(["Plano de recuperação atualizado"])
```

A gravação e a indexação se separam no momento em que a nota entra. A resposta ao cliente
sai assim que a transação fecha, e os dois indexadores seguem por conta própria, cada um com
sua fila no banco. No compose, eles rodam em laço; no Kubernetes, o componente Pulumi sobe um
Job por cofre pendente.

Isso torna o índice eventualmente consistente por escolha. Uma nota recém-salva pode não
aparecer na busca no segundo seguinte, e a alternativa seria prender a latência da escrita ao
tempo de gerar vetores.

Sem provedor de embedding, a ligação por sentido não acontece, e o resto do plano de
recuperação fica completo do mesmo jeito.

## Login e cadastro por provedor externo

```mermaid
flowchart TD
  A(["Pessoa escolhe entrar ou criar conta pelo provedor"]) --> F["Redireciona ao provedor"]
  F --> G["Pessoa autentica no provedor"]
  G --> H["Callback traz o identificador no provedor"]
  H --> Q{"Entrar ou criar conta?"}
  Q -- entrar --> I{"Identificador já vinculado a uma conta?"}
  I -- sim --> K(["Sessão aberta e login registrado na auditoria"])
  I -- não --> E(["Recusado, sugere criar conta"])
  Q -- criar conta --> R{"Provedor informou o email?"}
  R -- não --> S(["Recusado, falta o email"])
  R -- sim --> T{"Identificador ou email já têm conta?"}
  T -- sim --> U(["Recusado, já tem conta"])
  T -- não --> J{"Primeira conta da instância ou cadastro aberto para o provedor?"}
  J -- não --> V(["Recusado, cadastro fechado"])
  J -- sim --> L["Cria a conta com chave estável e a identidade sem senha"]
  L --> K
```

O login e o cadastro terminam na mesma abertura de sessão do login por senha. A diferença
entre instâncias é qual provedor está ligado, não qual código roda.

Autenticar no provedor prova identidade e não concede entrada. Quem não tem a identidade
vinculada não entra pelo login, e só ganha conta pelo cadastro quando o método está aberto.
Ninguém é vinculado a uma conta existente pelo email que o provedor informa; o vínculo nasce
nas configurações da conta, com a sessão aberta.

## Busca

```mermaid
flowchart TD
  A(["Consulta do usuário"]) --> B["A sessão define o tenant e os cofres do usuário"]
  B --> C["Consulta o índice léxico sob RLS"]
  C --> D{"Cliente pediu um cofre?"}
  D -- sim --> E["Recorta para esse cofre"]
  D -- não --> F["Mantém todos os cofres do usuário"]
  E --> H["Ordena pela relevância, com o título pesando mais"]
  F --> H
  H --> I(["Resultado com relevância e trecho destacado"])
```

A consulta corre sobre a sessão, que já carrega o tenant e os cofres do usuário. Isso dispensa
filtro de acesso escrito em cada consulta: a política de linha do banco recorta o resultado
antes da ordenação.

## Adicionar membro ao cofre

```mermaid
flowchart TD
  A(["Um membro escolhe uma conta do tenant"]) --> D{"Quem adiciona é viewer?"}
  D -- sim --> E(["Recusado, viewer só lê"])
  D -- não --> B{"Quem adiciona é membro do cofre?"}
  B -- não --> C(["Cofre fora do alcance"])
  B -- sim --> F{"A conta tem vínculo com o tenant?"}
  F -- não --> G(["Recusado, a conta entra antes pelo convite"])
  F -- sim --> H{"A conta já é membro do cofre?"}
  H -- sim --> I(["Recusado, já é membro"])
  H -- não --> J["Grava o membro e a linha de auditoria na mesma transação"]
  J --> K(["Passa a ver as notas do cofre"])
```

Não há email nem aceite: a conta já faz parte do tenant e entra no cofre na hora. Quem é de
fora passa antes pelo convite, que é onde o email acontece.

As verificações acontecem no serviço e de novo na política de linha do banco, então uma falha
no serviço sozinha não basta para gravar o membro.
