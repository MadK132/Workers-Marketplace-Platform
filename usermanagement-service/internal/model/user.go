package model

import "time"

type Role string

const (
	RoleCustomer Role = "customer"
	RoleWorker   Role = "worker"
	RoleAdmin    Role = "admin"
)

func (r Role) CanRegister() bool {
	return r == RoleCustomer || r == RoleWorker
}

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusBanned   Status = "banned"
)

type User struct {
	ID           int
	FullName     string
	Email        string
	Phone        *string
	PasswordHash string
	Role         Role
	Status       Status
	CreatedAt    time.Time
}
type WorkerProfile struct {
	ID                 int
	UserID             int
	VerificationStatus string
	IsAvailable        bool
}
type ExperienceLevel string

const (
	Junior ExperienceLevel = "junior"
	Middle ExperienceLevel = "middle"
	Senior ExperienceLevel = "senior"
)
