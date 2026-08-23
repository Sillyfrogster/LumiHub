# syntax=docker/dockerfile:1.7

FROM oven/bun:1.3.14 AS dependencies

WORKDIR /src/web

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

FROM node:24-bookworm-slim AS build

WORKDIR /src/web

COPY --from=dependencies /src/web/node_modules ./node_modules
COPY web/ ./

ARG ILLARIN_VERSION=development
ENV API_URL=http://api:8080 \
    ILLARIN_VERSION=${ILLARIN_VERSION} \
    NEXT_TELEMETRY_DISABLED=1

RUN node node_modules/next/dist/bin/next build

FROM node:24-bookworm-slim

WORKDIR /app

ENV HOSTNAME=0.0.0.0 \
    PORT=3000 \
    NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1

COPY --from=build --chown=node:node /src/web/public ./public
COPY --from=build --chown=node:node /src/web/.next/standalone ./
COPY --from=build --chown=node:node /src/web/.next/static ./.next/static

USER node

EXPOSE 3000

CMD ["node", "server.js"]
