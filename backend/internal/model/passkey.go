package model

import "time"

type Passkey struct {
	ID             uint64     `db:"id" json:"id"`
	UserID         uint64     `db:"user_id" json:"user_id"`
	Name           string     `db:"name" json:"name"`
	CredentialID   []byte     `db:"credential_id" json:"-"`
	PublicKey      []byte     `db:"public_key" json:"-"`
	Counter        uint32     `db:"counter" json:"-"`
	BackupEligible bool       `db:"backup_eligible" json:"-"`
	BackupState    bool       `db:"backup_state" json:"-"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	LastUsedAt     *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
}
