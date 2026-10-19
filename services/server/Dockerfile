# Build with: docker build -f services/server/Dockerfile -t one-server .
FROM node:24.21.0-bookworm-slim AS web
WORKDIR /src
RUN npm install -g pnpm@12.3.4
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/desktop/package.json apps/desktop/package.json
COPY apps/website/package.json apps/website/package.json
RUN pnpm install --frozen-lockfile
COPY apps/desktop apps/desktop
COPY scripts/server.mts scripts/server.mts
RUN node scripts/server.mts

FROM golang:1.27.1-bookworm AS build
WORKDIR /src
COPY go.work ./
COPY packages/one-core packages/one-core
COPY packages/one-client packages/one-client
COPY packages/cli packages/cli
COPY packages/templates packages/templates
COPY packages/agent-skills packages/agent-skills
COPY services/server services/server
COPY third_party third_party
COPY --from=web /src/services/server/internal/web/assets services/server/internal/web/assets
ARG VERSION=0.0.0-dev
RUN go run ./packages/cli/tools/sync-resources bundled
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/one-api ./services/server/cmd/one-api
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/one-api /usr/local/bin/one-api
COPY --from=build --chown=nonroot:nonroot /out/data /var/lib/one
ENV ONE_HOME=/var/lib/one ONE_SERVER_LISTEN=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/one-api", "serve"]
