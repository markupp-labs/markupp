# ADR-0052: Cadastro separado do login

## Status

Proposta

## Contexto

O login com credencial local criava a conta quando o email não existia: a primeira da
instalação como owner e, com o auto-registro ligado, as outras como conta sem tenant. Um erro de
digitação no email virava conta nova, e quem se cadastrava não sabia que estava se cadastrando

## Decisão

A conta nasce em `POST /accounts`, só sob /v1, que devolve a sessão com 201. O login só entra em
conta que já existe e recusa email desconhecido como credencial inválida. A primeira conta da
instalação nasce pelo cadastro mesmo com o auto-registro desligado, e o painel tem a tela "Criar
conta" ao lado do login

## Alternativas consideradas

- Manter o cadastro implícito no login: menos rotas, ao custo de conta criada por engano
- Login cria só a primeira conta e o cadastro cria as outras: dois caminhos para a mesma coisa

## Consequências

- Revê o ADR-0020 no caminho de criação da conta; o auto-registro continua desligado por padrão
- Instalação nova começa pelo cadastro, não pelo login
- Scripts e testes que entravam pelo primeiro login passam a cadastrar a conta antes
