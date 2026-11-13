# syntax=docker/dockerfile:1.7

# O Postgres do markupp: a imagem oficial em Alpine com o pgvector compilado nela
# (ADR-0055). Fica no Alpine porque trocar de imagem base troca a ordenação do texto
# (musl e glibc), e o banco que já existe ficaria com os índices de texto fora de ordem.
FROM postgres:17.9-alpine3.23

ARG PGVECTOR_VERSION=0.8.7
ARG PGVECTOR_SHA256=cac0b10c360f05b2d521200105ba3697e773d4cd3731f5a915a7e37ebe0bea85

# OPTFLAGS vazio tira o -march=native, para o binário rodar em qualquer CPU da
# arquitetura, e with_llvm=no dispensa o clang do JIT, que o pgvector não precisa.
RUN set -eux; \
  apk add --no-cache --virtual .pgvector-build build-base; \
  wget -q -O /tmp/pgvector.tar.gz "https://github.com/pgvector/pgvector/archive/refs/tags/v${PGVECTOR_VERSION}.tar.gz"; \
  echo "${PGVECTOR_SHA256}  /tmp/pgvector.tar.gz" | sha256sum -c -; \
  tar -xzf /tmp/pgvector.tar.gz -C /tmp; \
  make -C "/tmp/pgvector-${PGVECTOR_VERSION}" OPTFLAGS="" with_llvm=no; \
  make -C "/tmp/pgvector-${PGVECTOR_VERSION}" OPTFLAGS="" with_llvm=no install; \
  rm -rf /tmp/pgvector.tar.gz "/tmp/pgvector-${PGVECTOR_VERSION}"; \
  apk del .pgvector-build
