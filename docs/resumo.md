# Resumo do projeto

Markupp é um servidor de anotações em markdown que organiza a base sozinho, em vez de pedir
que o usuário organize.

O usuário escreve e salva. O servidor indexa, liga uma nota à outra e agrupa por tema, sem
nunca alterar o conteúdo nem o caminho do que foi escrito. A estrutura emerge do que está
lá, e o cliente mostra essa visão derivada no lugar da árvore de pastas.

Toda interação passa por uma API REST versionada. Painel web, plugin de editor e agente de
IA são clientes pares dessa API, e nenhum deles toca o armazenamento direto.

## O problema

Conhecimento pessoal e de time se espalha por arquivo solto, wiki abandonada e histórico de
conversa com agente. Quem escreve paga duas vezes, uma para registrar e outra para decidir
onde guardar. Meses depois ninguém acha.

Markupp aposta que a segunda cobrança é desnecessária. Uma base só, sem separação por
projeto, e a organização calculada pelo servidor.

## Como funciona

Cofre é a unidade de namespace e de acesso. A nota pertence a exatamente um cofre, e quem é
membro do cofre lê e escreve as notas dele. Todo usuário nasce com um cofre.

O dado durável vive no plano de registro, que é a fonte da verdade. Índice léxico, vetores,
grafo de ligações e agrupamentos vivem no plano de recuperação, derivado do registro e
reconstruível a qualquer momento. Apagar e reconstruir o índice inteiro é um comando, não um
incidente.

A busca combina dois ramos: o léxico, com relevância e trecho destacado, e o semântico,
sobre embeddings da nota.

## Edições

O Enterprise roda em Kubernetes na nuvem, com PostgreSQL gerenciado, isolamento por tenant
dentro do banco e autenticação por provedor externo.

O self-host de um nó roda por compose, com o mesmo código, um tenant só e a opção de gerar
embeddings por modelo local, sem depender de serviço de terceiro.
