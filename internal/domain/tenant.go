package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidInput = errors.New("invalid input")
	// ErrAuthFatal means credentials/lockout — stop polling this tenant to avoid further bans.
	ErrAuthFatal = errors.New("auth fatal")
)

// Tenant is a qrisgate app bound to one payment-provider account.
type Tenant struct {
	ID           string
	AppID        string // qrisgate app id
	Name         string
	Provider     string // gobiz
	Enabled      bool
	LoginMethod  string
	Email        string
	Password     string // never persisted for polling; may appear only in API request bodies
	Phone        string
	AccessToken  string
	RefreshToken string
	MerchantID   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
