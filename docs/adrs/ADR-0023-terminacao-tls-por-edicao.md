# ADR-0023: Terminação de TLS por edição

## Status

Aceita

## Contexto

O ADR-0005 tirou TLS do escopo do MVP. O acesso agora precisa ser cifrado nas duas edições, e
elas têm topologia diferente: o Enterprise tem balanceador na frente da aplicação, e o
self-host não tem nada na frente

## Decisão

No self-host o próprio servidor termina TLS, obtendo certificado do Let's Encrypt
automaticamente. No Enterprise o balanceador termina, com certificado do ACM, e o tráfego entre
o balanceador e a aplicação é HTTP

## Alternativas consideradas

- Servidor terminando TLS também no Enterprise: duplica o que o balanceador já faz, sem ganho,
  e obriga a distribuir e renovar certificado em cada instância
- Exigir proxy reverso no self-host: joga para quem hospeda uma configuração que o servidor
  resolve sozinho, e é a barreira que mais afasta quem instala pela primeira vez
- Certificado de arquivo como opção adicional no self-host: cobriria rede interna e certificado
  corporativo, e fica de fora por ora

## Consequências

- A instalação self-host precisa de domínio público e da porta 80 alcançável, porque é assim
  que o Let's Encrypt valida o domínio
- Instalação em rede interna, sem domínio público, fica sem TLS pelo servidor e volta a
  depender de proxy reverso
- No Enterprise o certificado renova sozinho pelo ACM, sem código na aplicação
- Atrás do balanceador a aplicação precisa confiar no cabeçalho de protocolo original, senão
  gera link com esquema errado
