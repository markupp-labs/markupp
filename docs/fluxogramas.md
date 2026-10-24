# Fluxogramas

Quatro fluxos, escolhidos porque cada um atravessa uma fronteira que os outros documentos
descrevem parada: a separação entre plano de registro e plano de recuperação, a troca com o
provedor de identidade, a fusão dos dois ramos de busca e a entrada num cofre, que depende do
vínculo com o tenant.

## Indexação de nota

```mermaid
flowchart TD
  A(["Cliente envia a nota"]) --> B["API REST valida caminho e conteúdo"]
  B --> C{"Caminho livre no cofre?"}
  C -- não --> D(["Erro de caminho ocupado"])
  C -- sim --> E["Grava a nota e a linha de auditoria na mesma transação"]
  E --> F["Avança o cursor de revisão"]
  F --> G(["Resposta ao cliente"])
  F --> H["Job do indexador determinístico sobe"]
  H --> I["Recalcula índice léxico, ligações e agrupamentos do cofre"]
  I --> J{"Provedor de embedding configurado?"}
  J -- não --> K(["Plano de recuperação atualizado sem vetores"])
  J -- sim --> L["Job do indexador de embedding gera os vetores"]
  L --> M["Grava os vetores em pgvector"]
  M --> N(["Plano de recuperação atualizado"])
  N --> O["Os Jobs escalam de volta a zero"]
  K --> O
```

A gravação e a indexação se separam no momento em que a nota entra. A resposta ao cliente
sai assim que a transação fecha, e a indexação segue em Job que subiu do zero.

Isso é o que torna o índice eventualmente consistente por escolha. Uma nota recém-salva pode
não aparecer na busca no segundo seguinte, e a alternativa seria prender a latência da
escrita ao tempo de gerar embeddings.

Sem provedor de embedding configurado, o ramo de vetores simplesmente não acontece e o
índice léxico fica completo do mesmo jeito.

## Autenticação por provedor externo

```mermaid
flowchart TD
  A(["Usuário pede acesso"]) --> B{"Provedor externo habilitado?"}
  B -- não --> C["Pede credencial local"]
  C --> D{"Credencial confere?"}
  D -- não --> E(["Acesso negado"])
  D -- sim --> K["Abre a sessão"]
  B -- sim --> F["Redireciona ao provedor OIDC"]
  F --> G["Usuário autentica no provedor"]
  G --> H["Callback traz o identificador do provedor"]
  H --> I{"Identificador já vinculado a uma conta?"}
  I -- sim --> K
  I -- não --> J{"Auto-registro ligado?"}
  J -- não --> E
  J -- sim --> L["Cria a conta com chave estável e o primeiro cofre"]
  L --> K
  K --> M(["Sessão aberta e operação registrada na auditoria"])
```

Os dois caminhos, credencial local e provedor externo, terminam na mesma abertura de sessão.
A diferença entre as edições é qual provedor está ligado, não qual código roda.

O ponto que costuma passar batido é o auto-registro desligado. Um usuário que autentica com
sucesso no provedor externo e não tem conta vinculada continua sem acesso, porque autenticar
prova identidade e não concede entrada.

## Busca com relevância

```mermaid
flowchart TD
  A(["Consulta do usuário"]) --> B["A sessão define o tenant e os cofres do usuário"]
  B --> C["Ramo léxico consulta o índice de texto sob RLS"]
  B --> D{"Provedor de embedding configurado?"}
  D -- sim --> F["Transforma a consulta em vetor"]
  F --> G["Ramo semântico consulta pgvector sob RLS"]
  D -- não --> E["Ramo semântico indisponível"]
  C --> H["Funde os rankings"]
  G --> H
  E --> H
  H --> I(["Resultado com relevância e trecho destacado"])
```

Os dois ramos correm sobre a mesma sessão, que já carrega o tenant e os cofres do usuário.
Isso é o que dispensa filtro de acesso escrito em cada consulta: a política de linha do
banco recorta o resultado antes da fusão.

Sem provedor de embedding, a busca não quebra, encolhe para o ramo léxico.

## Adicionar membro ao cofre

```mermaid
flowchart TD
  A(["Um membro escolhe uma conta do tenant"]) --> B{"Quem adiciona é membro do cofre?"}
  B -- não --> C(["Cofre fora do alcance"])
  B -- sim --> D{"Quem adiciona é viewer?"}
  D -- sim --> E(["Recusado, viewer só lê"])
  D -- não --> F{"A conta tem vínculo com o tenant?"}
  F -- não --> G(["Recusado, a conta entra antes pelo convite do tenant"])
  F -- sim --> H{"A conta já é membro do cofre?"}
  H -- sim --> I(["Recusado, já é membro"])
  H -- não --> J["Grava o membro e a linha de auditoria na mesma transação"]
  J --> K(["Passa a ver as notas do cofre"])
```

Não há email nem aceite: a conta já faz parte do tenant e entra no cofre na hora. Quem é de
fora passa antes pelo convite do tenant, que é onde o email acontece.

As verificações acontecem na rota e de novo na política de linha do banco, então uma falha
na rota sozinha não basta para gravar o membro.
