# Diagramas de blocos

![Arquitetura da edição Enterprise](diagramas-de-blocos.drawio.svg)

O sistema se divide em sete artefatos. A divisão segue perfil de recurso, não entidade de
domínio: nota, cofre, usuário e busca não são serviços, compartilham banco e caminho de
requisição. Dividir por entidade transformaria a escrita de uma nota em várias chamadas de
rede com transação distribuída.

## Clientes

Navegador, plugin de editor e agente de IA são pares sobre a mesma API REST versionada.
Nenhum deles toca o armazenamento, e nenhum tem rota privilegiada.

O navegador baixa o painel web do armazenamento de objeto com CDN e depois fala com a API
como qualquer outro cliente.

## Sempre ligado

A **API REST** atende pessoas, tem carga constante e é o caminho de entrada e saída de todo
conteúdo.

O **servidor MCP** atende agentes. Mesmo formato de requisição, rajada diferente. Fica
isolado para que um agente em laço não derrube a experiência de quem está digitando.

O **plano de controle** cuida de cobrança, provisionamento e plano. Tráfego baixo e postura
de segurança distinta dos outros dois.

## Escala a zero

O **indexador determinístico** gasta CPU em rajada e não carrega modelo. Roda como Job e
some quando não há nota para indexar.

O **indexador de embedding** tem perfil próprio, com acelerador quando o modelo é local. Sai
caro ligado, então escala a zero importa mais aqui do que em qualquer outro serviço.

O **worker de notificação** trata rajada com repetição, fora do caminho da requisição.

## Dados

Um PostgreSQL só, com o plano de registro e o plano de recuperação dentro dele. Os vetores
ficam em pgvector.

Os sete serviços compartilham esse banco, então o isolamento entre eles é de processo e de
escala, não de dado. Não são microsserviços no sentido de banco por serviço.

## Provedores externos

A API REST valida identidade contra o provedor OIDC habilitado. O indexador de embedding
gera vetores pelo Bedrock. O worker de notificação entrega email pelo SES. Os três são
trocáveis por configuração, e o self-host aponta para outra coisa sem mudar código.

## O que muda no self-host

O mesmo conjunto de imagens roda por compose num nó só. Sem cluster, sem CDN, o TLS termina
no próprio servidor, e o embedding pode vir de modelo local carregado no processo do
indexador.

A visão C4 do sistema é mantida no repositório do plugin (ADR-0030).
