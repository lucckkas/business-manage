package admin

import (
	"business-manage/internal/core"
	"context"
)

type Company struct {
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	CurrencyScale int    `json:"currencyScale"`
	Timezone      string `json:"timezone"`
}
type User struct {
	ID                      string `json:"id"`
	Username                string `json:"username"`
	Name                    string `json:"name"`
	Role                    string `json:"role"`
	Active                  bool   `json:"active"`
	PasswordHash            string `json:"-"`
	CreatedAt               string `json:"createdAt"`
	CanApplyManualDiscounts bool   `json:"canApplyManualDiscounts"`
	CanRecordCashExpenses   bool   `json:"canRecordCashExpenses"`
}

func (u User) CanDiscount() bool { return u.Role == "admin" || u.CanApplyManualDiscounts }
func (u User) CanExpense() bool  { return u.Role == "admin" || u.CanRecordCashExpenses }

type Session struct {
	Hash      string
	UserID    string
	CSRFToken string
	ExpiresAt string
}
type Repository interface {
	Company(context.Context) (Company, error)
	CreateCompany(context.Context, Company) error
	UserByUsername(context.Context, string) (User, error)
	User(context.Context, string) (User, error)
	Users(context.Context, core.Query) (core.Page[User], error)
	CreateUser(context.Context, User) error
	SetActive(context.Context, string, bool) error
	ActiveAdmins(context.Context) (int, error)
	UpdatePassword(context.Context, string, string) error
	SaveSession(context.Context, Session) error
	Session(context.Context, string) (Session, error)
	DeleteSession(context.Context, string) error
	DeleteUserSessions(context.Context, string) error
	UpdateProfile(context.Context, string, string) error
	UpdateCompany(context.Context, Company) error
	SetDiscountPermission(context.Context, string, bool) error
	SetExpensePermission(context.Context, string, bool) error
}
