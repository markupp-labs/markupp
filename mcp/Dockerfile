# syntax=docker/dockerfile:1.7

FROM node:26-alpine AS builder
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json tsconfig.build.json ./
COPY src ./src
RUN npm run build && npm prune --omit=dev

FROM node:26-alpine
WORKDIR /app
COPY --from=builder /src/package.json ./
COPY --from=builder /src/node_modules ./node_modules
COPY --from=builder /src/dist ./dist
USER node
EXPOSE 3000
ENTRYPOINT ["node", "dist/bin.js"]
CMD ["http"]
