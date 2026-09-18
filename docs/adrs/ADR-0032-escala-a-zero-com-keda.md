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

KEDA roda no cluster. Os dois indexadores são ScaledJob, com scaler de PostgreSQL consultando o
cursor de revisão, e um Job nasce por lote pendente. O worker de notificação continua
Deployment, com ScaledObject levando as réplicas a zero enquanto não houver notificação na fila
de saída

## Alternativas consideradas

- CronJob de minuto em minuto: não precisa de componente novo e o desenho fica exato, ao custo
  de acordar mesmo sem nota para indexar e de um piso de um minuto na latência
- Indexador sempre ligado: elimina a pergunta do gatilho, e paga recurso ocioso, que é
  exatamente o que o ADR-0018 recusou
- Broker de mensagens: resolve gatilho e ordem de uma vez, e é o componente que o ADR-0018
  adiou até o custo do polling ser medido e doer

## Consequências

- Fecha a lacuna do ADR-0027, que prometia escala a zero sem dizer quem dispara
- Continua sendo consulta ao cursor de revisão, então não contraria o ADR-0018 nem o ADR-0021.
  O polling sai da aplicação e passa para a plataforma
- KEDA é componente novo para instalar, versionar e operar no cluster
- O self-host por compose não tem Kubernetes nem KEDA, então o indexador precisa de um modo de
  laço além do modo de execução única que o Job usa
- A latência de indexação passa a ter estimativa, que é o intervalo de consulta do scaler
- Um Job por lote é um pod por lote, com o custo de partida que isso carrega
- KEDA não está instalado no cluster da disciplina, que hoje tem Gateway API, Cilium,
  cert-manager e Longhorn. Sem ele, e sem metrics-server para o ScaledObject, esta decisão
  descreve o alvo e não o que roda. Instalar é por Helm, que já existe nas máquinas
