# ADR-0032: Escala a zero com KEDA

## Status

Aceita

## Contexto

O ADR-0027 diz que a indexação roda como Job com escala a zero, e não diz quem cria o Job. Job
do Kubernetes roda até terminar e para, então alguém precisa criá-lo a cada lote.

O ADR-0018 rejeitou broker de mensagens e o ADR-0021 rejeitou fila dedicada, os dois
condicionando a entrada a medir o custo primeiro. Sem um mecanismo decidido, "escala a zero" é
promessa sem dono e a latência de indexação não tem estimativa

## Decisão

KEDA roda no cluster. Os dois indexadores são ScaledJob, com scaler de PostgreSQL contando os
cofres com nota pendente, e um Job nasce por cofre com mudança pendente. O worker de notificação continua
Deployment, com ScaledObject levando as réplicas a zero enquanto não houver notificação
pendente

A unidade de trabalho é o cofre, onde o dado derivado nasce (ADR-0024). Cada escrita dá à nota
uma revisão nova, e o índice guarda a revisão que indexou. Nota pendente é a que não tem linha no
índice ou tem revisão diferente, comparada nota a nota. Um número único por cofre perderia a
escrita que pega revisão antes de outra e confirma depois dela

A consulta do scaler conta os cofres com nota pendente, e cada Job reserva um deles travando a
linha de reserva do cofre com `SELECT ... FOR UPDATE SKIP LOCKED`. Um cofre já reservado é
pulado, então dois Jobs nunca indexam o mesmo lote, e Jobs em paralelo trabalham em cofres
diferentes

## Alternativas consideradas

- CronJob de minuto em minuto: não precisa de componente novo e o desenho fica exato, ao custo
  de acordar mesmo sem nota para indexar e de um piso de um minuto na latência
- Indexador sempre ligado: elimina a pergunta do gatilho, e paga recurso ocioso, que é
  exatamente o que o ADR-0018 recusou
- Um Job por vez, com `maxReplicaCount` 1: elimina a concorrência, e coloca a indexação de
  todos os cofres numa fila só
- Advisory lock por tenant: paraleliza entre tenants, e cofres do mesmo tenant esperam um ao
  outro sem precisar
- Broker de mensagens: resolve gatilho e ordem de uma vez, e é o componente que o ADR-0018
  adiou até o custo do polling ser medido e doer

## Consequências

- Fecha a lacuna do ADR-0027, que prometia escala a zero sem dizer quem dispara
- Continua sendo consulta ao banco, então não contraria o ADR-0018 nem o ADR-0021. O polling sai
  da aplicação e passa para a plataforma
- A consulta do scaler compara notas e índice a cada intervalo, custo que cresce com a base
- KEDA é componente novo para instalar, versionar e operar no cluster
- O self-host por compose não tem Kubernetes nem KEDA, então o indexador precisa de um modo de
  laço além do modo de execução única que o Job usa
- A latência de indexação passa a ter estimativa, que é o intervalo de consulta do scaler
- Um Job por lote é um pod por lote, com o custo de partida que isso carrega
- A reserva dura a transação: Job que morre no meio solta a trava, e o cofre volta a ser pego
  pelo próximo Job
- Um cofre grande não se divide entre Jobs, porque o paralelismo é por cofre
- KEDA vira pré-requisito do cluster, instalado por Helm, e o kind do CI o instala junto com os
  outros. O componente Pulumi deixa desligar o indexador em cluster sem KEDA. O próprio KEDA
  serve as métricas do scaler de PostgreSQL, então metrics-server só entra se algum serviço
  escalar por CPU ou memória
