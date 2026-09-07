# syntax=docker/dockerfile:1

# ---------- Tahap 1: kompilasi ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Unduh dependensi lebih dahulu agar lapisan ini dipakai ulang selama go.mod tidak berubah.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Binary statis tanpa ketergantungan pustaka sistem, dengan simbol debug dibuang.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/lms .

# ---------- Tahap 2: berkas pendukung ----------
FROM alpine:3.20 AS runtime-deps
RUN apk add --no-cache ca-certificates tzdata

# ---------- Tahap 3: image akhir ----------
FROM alpine:3.20

# Zona waktu dibutuhkan karena penjadwal laporan memakai waktu Asia/Jakarta.
ENV TZ=Asia/Jakarta

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -g 10001 -S lms \
    && adduser -u 10001 -S lms -G lms

WORKDIR /app

COPY --from=builder /out/lms /app/lms
# Templat soal pilihan ganda dibaca saat berkas soal diunggah.
COPY --from=builder /src/template_soal_pilihan_ganda.docx /app/template_soal_pilihan_ganda.docx

# Direktori unggahan dipasang sebagai volume agar berkas tidak hilang saat container diganti.
RUN mkdir -p /app/uploads && chown -R lms:lms /app

USER lms

EXPOSE 7777

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:${PORT:-7777}/api/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/lms"]
