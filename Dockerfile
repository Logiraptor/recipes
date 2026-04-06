FROM golang:1.25 AS build

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/trmnl-recipe ./cmd/trmnl-recipe

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/trmnl-recipe /trmnl-recipe

ENTRYPOINT ["/trmnl-recipe"]
