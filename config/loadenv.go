package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	DSN     string
	API_KEY string
	PORT    string

	// JWT_SECRET dipakai untuk menandatangani access token (HMAC-SHA256).
	JWT_SECRET []byte

	// ACCESS_TOKEN_TTL umur access token, pendek karena tidak bisa dicabut.
	ACCESS_TOKEN_TTL time.Duration

	// REFRESH_TOKEN_TTL umur refresh token, panjang tapi bisa dicabut lewat database.
	REFRESH_TOKEN_TTL time.Duration

	// MAX_LOGIN_ATTEMPTS jumlah gagal login sebelum akun dikunci sementara.
	MAX_LOGIN_ATTEMPTS int

	// LOGIN_LOCK_DURATION lama akun dikunci setelah melewati batas gagal login.
	LOGIN_LOCK_DURATION time.Duration

	// CORS_ORIGINS origin browser yang boleh memanggil API.
	CORS_ORIGINS []string

	// RATE_LIMIT_RPS dan RATE_LIMIT_BURST batas request per IP untuk endpoint /v1.
	RATE_LIMIT_RPS   int
	RATE_LIMIT_BURST int

	// TRUSTED_PROXIES daftar proxy yang header X-Forwarded-For-nya dipercaya.
	// Kosong berarti percaya semua (default Gin).
	TRUSTED_PROXIES []string
)

func LoadEnvVariables() {
	// File .env hanya dipakai saat development. Di production variabel biasanya
	// sudah diinject platform, jadi ketiadaan file ini bukan error.
	if err := godotenv.Load(); err != nil {
		log.Println("Tidak ada file .env, memakai environment variable yang ada")
	}

	DSN = os.Getenv("DATABASE_DSN")

	// Tanpa pengecekan ini, key yang kosong akan cocok dengan header kosong dan
	// seluruh endpoint /v1 jadi terbuka.
	API_KEY = os.Getenv("SECRET_SELF_API_KEY")
	if API_KEY == "" {
		log.Fatal("SECRET_SELF_API_KEY wajib diisi")
	}

	PORT = getEnv("PORT", "8000")

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET wajib diisi minimal 32 karakter")
	}
	JWT_SECRET = []byte(secret)

	ACCESS_TOKEN_TTL = getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	REFRESH_TOKEN_TTL = getEnvDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	MAX_LOGIN_ATTEMPTS = getEnvInt("MAX_LOGIN_ATTEMPTS", 5)
	LOGIN_LOCK_DURATION = getEnvDuration("LOGIN_LOCK_DURATION", 15*time.Minute)

	CORS_ORIGINS = getEnvList("CORS_ORIGINS", []string{
		"http://localhost:5173",
		"http://localhost:4173",
		"https://mangainaja.my.id",
	})

	// Satu halaman frontend bisa memicu beberapa request, jadi burst-nya longgar.
	RATE_LIMIT_RPS = getEnvInt("RATE_LIMIT_RPS", 10)
	RATE_LIMIT_BURST = getEnvInt("RATE_LIMIT_BURST", 30)

	TRUSTED_PROXIES = getEnvList("TRUSTED_PROXIES", nil)
}

// getEnvList membaca daftar yang dipisah koma, mis. "a.com, b.com".
func getEnvList(key string, fallback []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	parts := strings.Split(v, ",")
	list := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			list = append(list, trimmed)
		}
	}

	return list
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("Nilai %s tidak valid (%s), memakai default %d", key, v, fallback)
		return fallback
	}

	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("Nilai %s tidak valid (%s), memakai default %s", key, v, fallback)
		return fallback
	}

	return parsed
}
