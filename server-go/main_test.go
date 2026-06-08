package main

import (
	"testing"

	"auction-system/server-go/internal/realtime"
)

func TestNewRealtimeProviderFallsBackWhenMySQLUnavailable(t *testing.T) {
	t.Setenv("MYSQL_DSN", "invalid-dsn")

	provider, closeProvider := newRealtimeProvider()
	defer closeProvider()

	if _, ok := provider.(realtime.StaticProvider); !ok {
		t.Fatalf("expected StaticProvider fallback, got %T", provider)
	}
}
