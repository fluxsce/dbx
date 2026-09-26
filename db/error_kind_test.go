package db

import "testing"

func TestErrorKindRetryable(t *testing.T) {
	if ErrorUnique.Retryable() || ErrorOther.Retryable() {
		t.Fatal("unique and other are not retryable")
	}
	for _, k := range []ErrorKind{ErrorDeadlock, ErrorLock, ErrorSerialization} {
		if !k.Retryable() {
			t.Fatalf("%s", k)
		}
	}
	if ErrorUnique != "unique" || ErrorDeadlock != "deadlock" || ErrorLock != "lock" || ErrorSerialization != "serialization" || ErrorOther != "other" {
		t.Fatalf("%q %q %q %q %q", ErrorUnique, ErrorDeadlock, ErrorLock, ErrorSerialization, ErrorOther)
	}
	if ErrorKind("").String() != "other" {
		t.Fatal("zero value")
	}
	if ((*DB)(nil)).Classify(errSample) != ErrorOther {
		t.Fatal("nil session")
	}
}

var errSample = errString("x")

type errString string

func (e errString) Error() string { return string(e) }
