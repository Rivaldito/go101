package errwrap

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a user is not found in the store.
var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
)

// User represents a user in the system.
type User struct {
	ID    int
	Email string
}

// Store represents a simple in-memory user store.
type Store struct {
	users map[int]User
}

// NewStore creates a new Store with some initial users.
func NewStore() *Store {
	return &Store{
		users: map[int]User{1: {ID: 1, Email: "rivaldito@example.com"}},
	}
}

// FindUserByID retrieves a user by their ID, returning an error if the user is not found.
func (s *Store) FindUserByID(id int) (User, error) {
	u, ok := s.users[id]
	if !ok {
		return User{}, fmt.Errorf("store.FindUserByID id=%d: %w", id, ErrNotFound)
	}
	return u, nil
}

// Service holds the business logic on top of the Store.
type Service struct {
	store *Store
}

// NewService creates a new Service with the given Store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// GetUserByID retrieves a user's email by their ID, returning an error if the user is not found.
func (s *Service) GetUserByID(id int) (string, error) {
	user, err := s.store.FindUserByID(id)
	if err != nil {
		return "", fmt.Errorf("service.GetUserByID id=%d: %w", id, err)
	}
	return user.Email, nil
}

// GetUserByIDOpaque retrieves a user's email by their ID, returning an error if the user is not found.
func (s *Service) GetUserByIDOpaque(id int) (string, error) {
	user, err := s.store.FindUserByID(id)
	if err != nil {
		return "", fmt.Errorf("service.GetUserByIDOpaque id=%d: %v", id, err)
	}
	return user.Email, nil
}

// ValidationError represents an error that occurs when validating a user's input.
type ValidationError struct {
	Field  string
	Reason string
}

// Error implements the error interface for ValidationError.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed on %q: %s", e.Field, e.Reason)
}

// ValidateEmail checks if the provided email is valid, returning a ValidationError if it is not.
func ValidateEmail(email string) error {
	if email == "" {
		return &ValidationError{Field: "email", Reason: "empty"}
	}
	return nil
}

// UpdateEmail updates a user's email, returning an error if the email is invalid or the user is not found.
func (s *Service) UpdateEmail(id int, email string) error {
	if err := ValidateEmail(email); err != nil {
		return fmt.Errorf("service.UpdateEmail id=%d: %w", id, err)
	}
	// Simulate updating the email in the store
	if _, err := s.store.FindUserByID(id); err != nil {
		return fmt.Errorf("service.UpdateEmail id=%d: %w", id, err)
	}
	return nil
}

// Retryable is implemented by errors that know whether the operation can be retried.
type Retryable interface {
	error
	Retryable() bool
}

// TimeoutError is a retryable error.
type TimeoutError struct{ Op string }

func (e *TimeoutError) Error() string   { return e.Op + ": timeout" }
func (e *TimeoutError) Retryable() bool { return true }

// ShouldRetry reports whether any error in the chain says it is retryable.
func ShouldRetry(err error) bool {
	var r Retryable
	return errors.As(err, &r) && r.Retryable()
}

// QueryError is a database-like error that wraps its cause.
type QueryError struct {
	Query string
	Code  int
	Err   error
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query %q (code %d): %v", e.Query, e.Code, e.Err)
}

// Unwrap lets errors.Is/As reach the cause.
func (e *QueryError) Unwrap() error { return e.Err }

// Is makes a 404 equivalent to ErrNotFound.
func (e *QueryError) Is(target error) bool {
	return target == ErrNotFound && e.Code == 404
}

// RateLimitError matches any other *RateLimitError in errors.Is, whatever its fields.
type RateLimitError struct{ RetryAfter int }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %ds", e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	_, ok := target.(*RateLimitError)
	return ok
}

// ValidateUser collects every validation problem instead of stopping at the first one.
func ValidateUser(u User) error {
	var errs []error
	if u.ID <= 0 {
		errs = append(errs, &ValidationError{Field: "id", Reason: "must be positive"})
	}
	if err := ValidateEmail(u.Email); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...) // nil if there are no errors
}

// DoubleWrap wraps two errors at once.
func DoubleWrap() error {
	return fmt.Errorf("both: %w and %w", ErrNotFound, ErrUnauthorized)
}

// Walk returns the message of every error in the tree, depth first, left to right.
func Walk(err error) []string {
	var layers []string
	stack := []error{err}
	for len(stack) > 0 {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if e == nil {
			continue
		}
		layers = append(layers, e.Error())
		switch u := e.(type) {
		case interface{ Unwrap() error }:
			stack = append(stack, u.Unwrap())
		case interface{ Unwrap() []error }:
			children := u.Unwrap()
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		}
	}
	return layers
}
