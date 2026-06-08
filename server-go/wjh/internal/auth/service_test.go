package auth

import "testing"

func TestMockToken(t *testing.T) {
	got := MockToken("seller", 1)
	if got != "mock-token-seller-001" {
		t.Fatalf("unexpected token: %s", got)
	}
	got = MockToken("user", 1024)
	if got != "mock-token-user-1024" {
		t.Fatalf("unexpected token: %s", got)
	}
}

func TestMockKind(t *testing.T) {
	tests := map[string]string{
		"主播阿明": "seller",
		"商家小王": "seller",
		"卖家老李": "seller",
		"买家张三": "user",
	}
	for nickname, want := range tests {
		if got := mockKind(nickname); got != want {
			t.Fatalf("mockKind(%q)=%q, want %q", nickname, got, want)
		}
	}
}
