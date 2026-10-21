# ADR-0044: Rate limit nas rotas de conta

## Status

Aceita

## Contexto

O ADR-0005 deixou rate limit fora do MVP. Com a API exposta, login, refresh, aceite de convite,
verificação de email e troca de senha ficam abertos a tentativa de senha em massa e a disparo
de email em nome de terceiros

## Decisão

Essas rotas aceitam 5 tentativas por minuto por IP e, quando o corpo traz email, 5 por minuto
por email, contadas por rota numa janela fixa na memória do processo. O refresh conta só por
IP. Acima do limite, a API responde 429 `rate_limited` com Retry-After. O IP é o da conexão,
com IPv6 agrupado pelo prefixo /64, e o limite se ajusta em `MARKUPP_RATE_LIMIT_PER_MINUTE`.
Quando a conexão vem de um proxy listado em `MARKUPP_TRUSTED_PROXIES`, o IP é o primeiro do
X-Forwarded-For, lido da direita para a esquerda, que não é proxy confiável. Header vazio ou
malformado fica com o IP da conexão

## Alternativas consideradas

- Contagem compartilhada no PostgreSQL ou num Redis: limite exato entre réplicas, ao custo de
  escrita a cada tentativa ou de um serviço a mais
- Limite só por IP: quem troca de IP tenta a mesma conta sem parar
- Bloqueio da conta após falhas seguidas: trava o dono junto com o atacante

## Consequências

- Revê o ADR-0005 no item de rate limit
- Com várias réplicas, o limite vale por réplica, e reiniciar o processo zera a contagem
- Atrás de proxy fora de `MARKUPP_TRUSTED_PROXIES`, todos os clientes dividem o IP do proxy.
  Um CIDR largo demais na lista deixa o cliente escolher o IP que conta
- Quem conhece um email consegue segurar o login dessa conta por um minuto de cada vez
