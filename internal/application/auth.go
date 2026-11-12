package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/audit"
	"business-manage/internal/modules/reporting"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func digest(value string) string { s := sha256.Sum256([]byte(value)); return hex.EncodeToString(s[:]) }
func validatePassword(password string, allowWeak ...bool) error {
	if utf8.RuneCountInString(password) > 128 {
		return core.Invalid("La contraseña admite hasta 128 caracteres.")
	}
	if utf8.RuneCountInString(password) < 12 && (len(allowWeak) == 0 || !allowWeak[0]) {
		return core.Invalid("Acepta la advertencia para usar una contraseña de menos de 12 caracteres o vacía.")
	}
	return nil
}
func hashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 1, 32)
	return "$argon2id$v=19$m=65536,t=1,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash)
}
func checkPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[3] != "m=65536,t=1,p=1" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 1, 64*1024, 1, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
func username(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < 3 || len(value) > 64 {
		return "", core.Invalid("El usuario debe tener entre 3 y 64 caracteres.")
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' && r != '_' && r != '@' {
			return "", core.Invalid("El usuario contiene caracteres inválidos.")
		}
	}
	return value, nil
}
func auditEntry(actor, action, entity string) audit.Entry {
	return audit.Entry{ID: core.ID(), ActorID: actor, Action: action, EntityID: entity, CreatedAt: core.Now()}
}

type SetupRequest struct {
	Company           admin.Company `json:"company"`
	Username          string        `json:"username"`
	Name              string        `json:"name"`
	Password          string        `json:"password"`
	AllowWeakPassword bool          `json:"allowWeakPassword"`
}

func (s *Service) Setup(ctx context.Context, r SetupRequest) error {
	r.Company.Name = strings.TrimSpace(r.Company.Name)
	r.Name = strings.TrimSpace(r.Name)
	if len(r.Company.Name) < 2 || len(r.Company.Name) > 160 || len(r.Name) < 2 || len(r.Name) > 120 {
		return core.Invalid("Indica el nombre de la empresa y del administrador.")
	}
	// Supported currencies are explicit so their minor-unit precision cannot be misconfigured.
	scales := map[string]int{"CLP": 0, "USD": 2, "EUR": 2, "PEN": 2, "MXN": 2, "ARS": 2, "COP": 2, "UYU": 2, "BOB": 2, "BRL": 2}
	scale, ok := scales[r.Company.Currency]
	if !ok {
		return core.Invalid("Selecciona una moneda admitida.")
	}
	r.Company.CurrencyScale = scale
	if _, err := time.LoadLocation(r.Company.Timezone); err != nil {
		return core.Invalid("La zona horaria no es válida.")
	}
	user, err := username(r.Username)
	if err != nil {
		return err
	}
	if err = validatePassword(r.Password, r.AllowWeakPassword); err != nil {
		return err
	}
	u := admin.User{ID: core.ID(), Username: user, Name: r.Name, Role: "admin", Active: true, PasswordHash: hashPassword(r.Password), CreatedAt: core.Now()}
	return s.Store.WithinTx(ctx, func(p Ports) error {
		if _, err := p.Admin.Company(ctx); err == nil {
			return core.Conflict("La empresa ya está configurada.")
		} else if !core.HasCode(err, "not_found") {
			return err
		}
		if err := p.Admin.CreateCompany(ctx, r.Company); err != nil {
			return err
		}
		if err := p.Admin.CreateUser(ctx, u); err != nil {
			return err
		}
		return p.Audit.Append(ctx, auditEntry(u.ID, "company.setup", "company"))
	})
}

type LoginResult struct {
	User      admin.User    `json:"user"`
	Company   admin.Company `json:"company"`
	CSRFToken string        `json:"csrfToken"`
	Token     string        `json:"-"`
	Expires   time.Time     `json:"-"`
}

func (s *Service) Login(ctx context.Context, name, password string) (LoginResult, error) {
	if utf8.RuneCountInString(password) > 128 {
		return LoginResult{}, core.Unauthorized
	}
	u, err := s.Store.Read().Admin.UserByUsername(ctx, strings.ToLower(strings.TrimSpace(name)))
	if err != nil && !core.HasCode(err, "not_found") {
		return LoginResult{}, err
	}
	if err != nil || !u.Active { // Do equivalent work for unknown and disabled accounts.
		_ = checkPassword(password, "$argon2id$v=19$m=65536,t=1,p=1$MDEyMzQ1Njc4OWFiY2RlZg$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		return LoginResult{}, core.Unauthorized
	}
	if !checkPassword(password, u.PasswordHash) {
		return LoginResult{}, core.Unauthorized
	}
	token := core.Token()
	csrf := core.Token()
	expires := time.Now().UTC().Add(12 * time.Hour)
	err = s.Store.WithinTx(ctx, func(p Ports) error {
		current, e := p.Admin.User(ctx, u.ID)
		if e != nil {
			return e
		}
		if !current.Active || current.PasswordHash != u.PasswordHash {
			return core.Unauthorized
		}
		if e = p.Admin.SaveSession(ctx, admin.Session{Hash: digest(token), UserID: u.ID, CSRFToken: csrf, ExpiresAt: expires.Format(time.RFC3339Nano)}); e != nil {
			return e
		}
		return p.Audit.Append(ctx, auditEntry(u.ID, "session.login", u.ID))
	})
	if err != nil {
		return LoginResult{}, err
	}
	c, err := s.Store.Read().Admin.Company(ctx)
	return LoginResult{User: u, Company: c, CSRFToken: csrf, Token: token, Expires: expires}, err
}
func (s *Service) Authenticate(ctx context.Context, token string) (admin.User, admin.Session, error) {
	if len(token) != 64 {
		return admin.User{}, admin.Session{}, core.Unauthorized
	}
	session, err := s.Store.Read().Admin.Session(ctx, digest(token))
	if err != nil {
		if !core.HasCode(err, "not_found") {
			return admin.User{}, session, err
		}
		return admin.User{}, session, core.Unauthorized
	}
	expires, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil || time.Now().After(expires) {
		return admin.User{}, session, core.Unauthorized
	}
	u, err := s.Store.Read().Admin.User(ctx, session.UserID)
	if err != nil && !core.HasCode(err, "not_found") {
		return u, session, err
	}
	if err != nil || !u.Active {
		return u, session, core.Unauthorized
	}
	return u, session, nil
}
func (s *Service) Logout(ctx context.Context, session admin.Session) error {
	return s.Store.WithinTx(ctx, func(p Ports) error { return p.Admin.DeleteSession(ctx, session.Hash) })
}

type CreateUserRequest struct {
	Username                string `json:"username"`
	Name                    string `json:"name"`
	Password                string `json:"password"`
	AllowWeakPassword       bool   `json:"allowWeakPassword"`
	Role                    string `json:"role"`
	CanApplyManualDiscounts bool   `json:"canApplyManualDiscounts"`
	CanRecordCashExpenses   bool   `json:"canRecordCashExpenses"`
}

func (s *Service) CreateUser(ctx context.Context, actor admin.User, r CreateUserRequest) (admin.User, error) {
	if actor.Role != "admin" {
		return admin.User{}, core.Forbidden
	}
	name, err := username(r.Username)
	if err != nil {
		return admin.User{}, err
	}
	if err = validatePassword(r.Password, r.AllowWeakPassword); err != nil {
		return admin.User{}, err
	}
	r.Name = strings.TrimSpace(r.Name)
	if len(r.Name) < 2 || len(r.Name) > 120 {
		return admin.User{}, core.Invalid("Indica un nombre de 2 a 120 caracteres.")
	}
	if r.Role != "admin" && r.Role != "operator" {
		return admin.User{}, core.Invalid("Rol no válido.")
	}
	u := admin.User{ID: core.ID(), Username: name, Name: r.Name, Role: r.Role, Active: true, PasswordHash: hashPassword(r.Password), CreatedAt: core.Now(), CanApplyManualDiscounts: r.Role == "admin" || r.CanApplyManualDiscounts, CanRecordCashExpenses: r.Role == "admin" || r.CanRecordCashExpenses}
	err = s.Store.WithinTx(ctx, func(p Ports) error {
		if e := p.Admin.CreateUser(ctx, u); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "user.create", u.ID)
		entry.Details = map[string]any{"name": u.Name, "username": u.Username, "role": u.Role, "canApplyManualDiscounts": u.CanApplyManualDiscounts, "canRecordCashExpenses": u.CanRecordCashExpenses, "weakCredentialAccepted": utf8.RuneCountInString(r.Password) < 12}
		return p.Audit.Append(ctx, entry)
	})
	return u, err
}
func (s *Service) SetUserActive(ctx context.Context, actor admin.User, id string, active bool) error {
	if actor.Role != "admin" {
		return core.Forbidden
	}
	if id == actor.ID && !active {
		return core.Conflict("No puedes desactivar tu propia cuenta.")
	}
	return s.Store.WithinTx(ctx, func(p Ports) error {
		u, e := p.Admin.User(ctx, id)
		if e != nil {
			return e
		}
		if u.Role == "admin" && u.Active && !active {
			n, e := p.Admin.ActiveAdmins(ctx)
			if e != nil {
				return e
			}
			if n <= 1 {
				return core.Conflict("Debe quedar al menos un administrador activo.")
			}
		}
		if e = p.Admin.SetActive(ctx, id, active); e != nil {
			return e
		}
		if !active {
			if e = p.Admin.DeleteUserSessions(ctx, id); e != nil {
				return e
			}
		}
		entry := auditEntry(actor.ID, "user.active", id)
		entry.Details = map[string]any{"before": u.Active, "after": active}
		return p.Audit.Append(ctx, entry)
	})
}
func (s *Service) ResetPassword(ctx context.Context, name, password string, allowWeak ...bool) error {
	if err := validatePassword(password, allowWeak...); err != nil {
		return err
	}
	hash := hashPassword(password)
	return s.Store.WithinTx(ctx, func(p Ports) error {
		u, e := p.Admin.UserByUsername(ctx, strings.ToLower(strings.TrimSpace(name)))
		if e != nil {
			return e
		}
		if e = p.Admin.UpdatePassword(ctx, u.ID, hash); e != nil {
			return e
		}
		if e = p.Admin.DeleteUserSessions(ctx, u.ID); e != nil {
			return e
		}
		return p.Audit.Append(ctx, auditEntry(u.ID, "user.password_reset_local", u.ID))
	})
}
func (s *Service) Summary(ctx context.Context) (reporting.Summary, error) {
	c, err := s.Store.Read().Admin.Company(ctx)
	if err != nil {
		return reporting.Summary{}, err
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return reporting.Summary{}, fmt.Errorf("company timezone: %w", err)
	}
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	return s.Store.Read().Reports.Summary(ctx, start.UTC().Format("2006-01-02T15:04:05.000000000Z"), end.UTC().Format("2006-01-02T15:04:05.000000000Z"))
}
