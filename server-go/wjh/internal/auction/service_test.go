package auction

import (
	"testing"
	"time"
)

func TestValidateCreateRejectsInvalidPrice(t *testing.T) {
	service := NewService(nil, time.Local)
	_, err := service.validateCreate(1, CreateRequest{
		Title:           "测试拍卖",
		StartPrice:      100,
		PriceStep:       0,
		StartTime:       time.Now().Add(time.Hour).Format(time.RFC3339Nano),
		DurationSeconds: 300,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateCreateDefaultsExtendFields(t *testing.T) {
	service := NewService(nil, time.Local)
	params, err := service.validateCreate(1, CreateRequest{
		Title:           "测试拍卖",
		StartPrice:      0,
		PriceStep:       100,
		StartTime:       time.Now().Add(time.Hour).Format(time.RFC3339Nano),
		DurationSeconds: 300,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.ExtendSec != 30 || params.ExtendThreshold != 30 {
		t.Fatalf("unexpected defaults: %d/%d", params.ExtendSec, params.ExtendThreshold)
	}
	if params.EndAt.Sub(params.StartAt) != 300*time.Second {
		t.Fatalf("unexpected end time: %s", params.EndAt)
	}
}

func TestValidStatus(t *testing.T) {
	for _, status := range []string{"pending", "active", "ended", "cancelled"} {
		if !validStatus(status) {
			t.Fatalf("expected %s to be valid", status)
		}
	}
	if validStatus("deleted") {
		t.Fatal("unexpected valid status")
	}
}
