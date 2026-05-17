package domain

import "time"

// User 的 ID 是 app 层生成的 UUID（google/uuid），存储为 text。不写
// `type:uuid` 让 Gorm 按 dialect 选 text-affinity 默认——Postgres 用
// text 列，SQLite 同样 text 亲和——便于同一份 schema 在两种 driver 间
// portable。
type User struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Email        string    `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Name         string    `gorm:"size:120;not null" json:"name"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
