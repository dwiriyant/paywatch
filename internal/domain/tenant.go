package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidInput = errors.New("invalid input")
)

// Tenant is a qrisgate app bound to one payment-provider account.
type Tenant struct {
	ID          string
	AppID       string // qrisgate app id
	Name        string
	Provider    string // gobiz
	Enabled     bool
	LoginMethod string
	Email       string
	Password    string
	Phone       string
	AccessToken string
	MerchantID  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
