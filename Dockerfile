FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependency di-download di layer terpisah supaya build ulang setelah mengubah
# kode tidak perlu mengunduh ulang semuanya.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api .

# Distroless static sudah berisi sertifikat CA (untuk scraping HTTPS) dan data
# zona waktu, tanpa shell, dan berjalan sebagai user non-root.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /api /api

EXPOSE 8000

ENTRYPOINT ["/api"]
