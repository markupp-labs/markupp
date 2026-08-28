# ADR-0022: Registro de operações críticas

## Status

Aceita

## Contexto

É preciso saber quem fez o quê para análise posterior: login, criação e remoção de usuário,
mudança de permissão, mudança de plano, exclusão de nota e sobrescrita forçada em conflito. O
cursor de revisão e os tombstones registram que a nota mudou, mas não registram autor nem
operação administrativa

## Decisão

Tabela de auditoria no plano de registro, escrita na mesma transação da operação auditada, com
tenant, ator, ação, alvo, momento e resultado. Somente inserção: a aplicação não atualiza
nem exclui linha de auditoria

## Alternativas consideradas

- Derivar a auditoria do histórico de notas: não cobre operação administrativa nem login, e o
  histórico versionado ainda não existe
- Log estruturado em arquivo ou serviço externo: serve para observabilidade, mas é sujeito a
  perda e ruim de consultar por usuário, o que não atende análise posterior
- Escrever fora da transação da operação: operação bem-sucedida pode ficar sem registro
  justamente no caso em que o registro importa

## Consequências

- A auditoria é dado durável do plano de registro (ADR-0012), entra no backup e não pode ser
  reconstruída, diferente de tudo no plano de recuperação
- Escrever na mesma transação soma a latência do registro à da operação, custo aceitável no
  volume esperado
- A tabela cresce sem limite e precisa de política de retenção e de particionamento por tempo
- O RLS do ADR-0016 vale também para a auditoria, senão um cliente enxerga ação de outro
- Somente inserção exige papel de banco sem permissão de update e delete nessa tabela
