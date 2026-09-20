package model_user

import (
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// User adalah pemilik akun API. Password disimpan sebagai hash bcrypt dan
// tidak pernah ikut ter-serialize ke JSON (tag `json:"-"`).
type User struct {
	gorm.Model
	Username string `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Email    string `gorm:"type:varchar(120);uniqueIndex;not null" json:"email"`
	Password string `gorm:"type:varchar(120);not null" json:"-"`
	Role     Role   `gorm:"type:varchar(20);not null;default:'user'" json:"role"`
	IsActive bool   `gorm:"not null;default:true" json:"is_active"`

	// Proteksi brute force: dihitung per akun, direset setiap login sukses.
	FailedLoginAttempts int        `gorm:"not null;default:0" json:"-"`
	LockedUntil         *time.Time `json:"-"`
	LastLoginAt         *time.Time `json:"last_login_at"`

	RefreshTokens  []RefreshToken   `gorm:"foreignKey:UserID" json:"-"`
	Bookmarks      []Bookmark       `gorm:"foreignKey:UserID" json:"-"`
	ReadingHistory []ReadingHistory `gorm:"foreignKey:UserID" json:"-"`
}

// IsLocked menandakan akun sedang dikunci sementara karena gagal login berulang.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// RefreshToken menyimpan sidik jari (SHA-256) dari refresh token, bukan token
// mentahnya, supaya bocornya database tidak langsung berarti akun bisa diambil alih.
type RefreshToken struct {
	gorm.Model
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	TokenHash string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	UserAgent string     `gorm:"type:varchar(255)" json:"user_agent"`
	IP        string     `gorm:"type:varchar(64)" json:"ip"`
}

// IsUsable true kalau token belum dicabut dan belum kedaluwarsa.
func (t *RefreshToken) IsUsable() bool {
	return t.RevokedAt == nil && t.ExpiresAt.After(time.Now())
}

// Bookmark adalah komik yang disimpan user. Title dan ImgUrl di-denormalisasi
// supaya daftar bookmark bisa ditampilkan tanpa join ke tabel komiks.
type Bookmark struct {
	gorm.Model
	UserID    uint   `gorm:"not null;uniqueIndex:idx_user_bookmark" json:"user_id"`
	KomikSlug string `gorm:"type:varchar(200);not null;uniqueIndex:idx_user_bookmark" json:"komik_slug"`
	Title     string `gorm:"type:varchar(255)" json:"title"`
	ImgUrl    string `gorm:"type:varchar(500)" json:"imgurl"`
}

// ReadingHistory menyimpan satu baris per (user, komik): chapter terakhir yang
// dibaca. Ditulis dengan upsert supaya tidak menumpuk baris per chapter.
type ReadingHistory struct {
	gorm.Model
	UserID       uint      `gorm:"not null;uniqueIndex:idx_user_history" json:"user_id"`
	KomikSlug    string    `gorm:"type:varchar(200);not null;uniqueIndex:idx_user_history" json:"komik_slug"`
	ChapterSlug  string    `gorm:"type:varchar(200);not null" json:"chapter_slug"`
	ChapterTitle string    `gorm:"type:varchar(255)" json:"chapter_title"`
	LastPanel    int       `gorm:"not null;default:0" json:"last_panel"`
	LastReadAt   time.Time `gorm:"index;not null" json:"last_read_at"`
}
