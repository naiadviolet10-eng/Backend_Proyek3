FROM golang:alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o main .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /app/main .
COPY --from=build /app/templates ./templates
COPY --from=build /app/static ./static
ENV PORT=8080
EXPOSE 8080
CMD ["./main"]
