FROM golang:1.25 AS build

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/trmnl-recipe ./cmd/trmnl-recipe

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/trmnl-recipe /trmnl-recipe

# The repo is the database: bake the data in and point the tool at it.
COPY recipes /data/recipes
COPY meal-plans /data/meal-plans
ENV RECIPES_ROOT=/data

ENTRYPOINT ["/trmnl-recipe"]
