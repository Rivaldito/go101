package errwrap

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestWrapKeepChan(t *testing.T) {
	// Init the service
	svc := NewService(NewStore())
	// Get a wrong user
	_, err := svc.GetUserByID(333)
	// If we got nil, something is really wrong with our minimal code
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// We want this error
	want := "service.GetUserByID id=333: store.FindUserByID id=333: not found"
	if err.Error() != want {
		t.Fatalf("err.Error() = %q; want %q", err.Error(), want)
	}

	t.Logf("the full error: %v", err) //in this case we use %v, because t.Logf dont suport %w, and we want to see the error chain
	t.Logf("inside the error: %v", errors.Unwrap(err))
	t.Logf("inside the inside error: %v", errors.Unwrap(errors.Unwrap(err)))
}

func TestWrapOpaque(t *testing.T) {
	// Init the service
	svc := NewService(NewStore())
	// Get a wrong user
	_, err := svc.GetUserByIDOpaque(333)
	// If we got nil, something is really wrong with our minimal code
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// We want this error
	want := "service.GetUserByIDOpaque id=333: store.FindUserByID id=333: not found"
	if err.Error() != want {
		t.Fatalf("err.Error() = %q; want %q", err.Error(), want)
	}

	t.Logf("the full error: %v", err) //in this case we use %v, because t.Logf dont suport %w, and we want to see the error chain
	t.Logf("inside the error: %v", errors.Unwrap(err))
	t.Logf("inside the inside error: %v", errors.Unwrap(errors.Unwrap(err)))
}

func TestErrorfConcreteTypes(t *testing.T) {
	//
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"without %w", fmt.Errorf("x: %v", ErrNotFound), "*errors.errorString"},
		{"with %w", fmt.Errorf("x: %w", ErrNotFound), "*fmt.wrapError"},
		{"with %w %w", fmt.Errorf("x: %w %w", ErrNotFound, ErrUnauthorized), "*fmt.wrapErrors"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			t.Log(reflect.TypeOf(c.err).String())
			if got := reflect.TypeOf(c.err).String(); got != c.want {
				t.Errorf("tipo = %s; want %s", got, c.want)
			}
		})
	}
}

// TestErrorsAs tests the errors.As function with a custom error type.Z
func TestErrorsAs(t *testing.T) {
	err := NewService(NewStore()).UpdateEmail(1, "")
	var ve *ValidationError
	t.Log(err)
	if !errors.As(err, &ve) {
		t.Fatalf("we expect *ValidationError in the chain; err = %v", err)
	}

	t.Log(ve.Field)
	t.Log(ve.Reason)
}

func TestNilIsNotWrapped(t *testing.T) {
	var err error
	if errors.Is(err, ErrNotFound) {
		t.Error("nil is not ErrNotFound")
	}
	if !errors.Is(nil, nil) {
		t.Error("errors.Is(nil, nil) should be true")
	}
}

func TestAsWithInterface(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"wrapped timeout", fmt.Errorf("client.Get: %w", &TimeoutError{Op: "dial"}), true},
		{"not found", fmt.Errorf("client.Get: %w", ErrNotFound), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldRetry(tt.err); got != tt.want {
				t.Errorf("ShouldRetry(%v) = %v; want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestCustomIs(t *testing.T) {
	cause := errors.New("no rows")
	err := fmt.Errorf("repo: %w", &QueryError{Query: "SELECT 1", Code: 404, Err: cause})

	if !errors.Is(err, ErrNotFound) {
		t.Error("Code 404 should be equivalent to ErrNotFound")
	}
	if !errors.Is(err, cause) {
		t.Error("the original cause should still be reachable through Unwrap")
	}
	var qe *QueryError
	if !errors.As(err, &qe) || qe.Code != 404 {
		t.Errorf("errors.As failed or wrong Code: %v", qe)
	}

	other := &QueryError{Query: "SELECT 1", Code: 500, Err: cause}
	if errors.Is(other, ErrNotFound) {
		t.Error("Code 500 should not be equivalent to ErrNotFound")
	}
}

func TestIsByType(t *testing.T) {
	err := fmt.Errorf("api: %w", &RateLimitError{RetryAfter: 30})
	if !errors.Is(err, &RateLimitError{}) {
		t.Error("any *RateLimitError should match")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("should not be ErrNotFound")
	}
	// Without an Is method, two different pointers are never ==.
	a, b := &TimeoutError{Op: "x"}, &TimeoutError{Op: "x"}
	if errors.Is(a, b) {
		t.Error("different pointers should not match without Is")
	}
}

func TestJoin(t *testing.T) {
	err := ValidateUser(User{ID: 0, Email: ""})
	if err == nil {
		t.Fatal("expected error")
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatal("errors.Join should implement Unwrap() []error")
	}
	if n := len(joined.Unwrap()); n != 2 {
		t.Errorf("len = %d; want 2", n)
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatal("errors.As should find the first ValidationError")
	}
	if ve.Field != "id" {
		t.Errorf("first Field = %q; want id", ve.Field)
	}
	if ValidateUser(User{ID: 1, Email: "x@y.z"}) != nil {
		t.Error("errors.Join with no errors must return nil")
	}
}

func TestMultiplePercentW(t *testing.T) {
	err := DoubleWrap()
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, ErrUnauthorized) {
		t.Errorf("both sentinels should be in the chain: %v", err)
	}
	if errors.Unwrap(err) != nil {
		t.Error("errors.Unwrap returns nil with several %w")
	}
}

func TestWalk(t *testing.T) {
	_, err := NewService(NewStore()).GetUserByID(42)
	got := Walk(err)
	want := []string{
		"service.GetUserByID id=42: store.FindUserByID id=42: not found",
		"store.FindUserByID id=42: not found",
		"not found",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Walk = %q; want %q", got, want)
	}
}

func TestWalkTree(t *testing.T) {
	err := fmt.Errorf("handler: %w", errors.Join(ErrNotFound, ErrUnauthorized))
	got := Walk(err)
	want := []string{
		"handler: not found\nunauthorized",
		"not found\nunauthorized",
		"not found",
		"unauthorized",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Walk = %q; want %q", got, want)
	}
}

func TestUpdateEmail(t *testing.T) {
	tests := []struct {
		name       string
		id         int
		email      string
		wantIs     error
		wantField  string
		wantNilErr bool
	}{
		{name: "ok", id: 1, email: "a@b.com", wantNilErr: true},
		{name: "empty email", id: 1, email: "", wantField: "email"},
		{name: "missing user", id: 99, email: "a@b.com", wantIs: ErrNotFound},
	}
	svc := NewService(NewStore())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.UpdateEmail(tt.id, tt.email)
			if tt.wantNilErr {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantIs != nil {
				assertErrorIs(t, err, tt.wantIs)
			}
			if tt.wantField != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected *ValidationError; err = %v", err)
				}
				if ve.Field != tt.wantField {
					t.Errorf("Field = %q; want %q", ve.Field, tt.wantField)
				}
			}
		})
	}
}

func assertErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("errors.Is(%v, %v) = false", err, target)
	}
}

func ExampleService_GetUserByID() {
	_, err := NewService(NewStore()).GetUserByID(42)
	fmt.Println(err)
	fmt.Println(errors.Is(err, ErrNotFound))
	// Output:
	// service.GetUserByID id=42: store.FindUserByID id=42: not found
	// true
}
