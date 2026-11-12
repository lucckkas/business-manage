// Package core contains database- and transport-independent primitives.
package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

type Error struct{ Code, Message string }

func (e *Error) Error() string      { return e.Message }
func Invalid(message string) error  { return &Error{"validation", message} }
func Conflict(message string) error { return &Error{"conflict", message} }

var (
	NotFound     = &Error{"not_found", "No se encontró el registro."}
	Unauthorized = &Error{"unauthorized", "Inicia sesión para continuar."}
	Forbidden    = &Error{"forbidden", "No tienes permiso para realizar esta operación."}
)

func HasCode(err error, code string) bool { var e *Error; return errors.As(err, &e) && e.Code == code }

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Now() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") }

type Page[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}
type Query struct {
	Search         string
	Page, PageSize int
	SellableOnly   bool
	OwnOnly        bool
	Action         string
	Kind           string
}

func (q Query) Normalized() Query {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	q.Search = strings.TrimSpace(q.Search)
	return q
}
func (q Query) Offset() int { return (q.Page - 1) * q.PageSize }

// Minor is an exact, non-negative monetary amount, serialized as a decimal string.
func Minor(value string) (int64, error) {
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return 0, Invalid("El importe debe expresarse como un entero positivo.")
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n > 1_000_000_000_000_000 {
		return 0, Invalid("El importe excede el máximo permitido.")
	}
	return n, nil
}

// Quantity uses thousandths; it never passes through floating-point arithmetic.
func Quantity(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || strings.Trim(parts[0], "0123456789") != "" {
		return 0, Invalid("Cantidad inválida; utiliza hasta tres decimales.")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" || len(fraction) > 3 || strings.Trim(fraction, "0123456789") != "" {
			return 0, Invalid("Cantidad inválida; utiliza hasta tres decimales.")
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 1_000_000_000 {
		return 0, Invalid("Cantidad fuera de rango.")
	}
	for len(fraction) < 3 {
		fraction += "0"
	}
	f, _ := strconv.ParseInt(fraction, 10, 64)
	return whole*1000 + f, nil
}
func QuantityText(q int64) string { return fmt.Sprintf("%d.%03d", q/1000, q%1000) }

// LineTotal rounds half up once per line, using big.Int to prevent intermediate overflow.
func LineTotal(price, quantity int64) (int64, error) {
	if price < 0 || quantity <= 0 {
		return 0, Invalid("El precio y la cantidad no son válidos.")
	}
	n := new(big.Int).Mul(big.NewInt(price), big.NewInt(quantity))
	n.Add(n, big.NewInt(500))
	n.Div(n, big.NewInt(1000))
	if !n.IsInt64() || n.Int64() > 1_000_000_000_000_000 {
		return 0, Invalid("El total excede el máximo permitido.")
	}
	return n.Int64(), nil
}
