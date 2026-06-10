package auction

import (
	"encoding/json"
	"time"
)

func mapAuction(row auctionRow, location *time.Location) Auction {
	images := []string{}
	if row.ImagesJSON != nil && *row.ImagesJSON != "" {
		_ = json.Unmarshal([]byte(*row.ImagesJSON), &images)
	}

	var leader *UserLite
	if row.CurrentLeaderID != nil {
		leader = &UserLite{
			ID:       *row.CurrentLeaderID,
			Nickname: deref(row.CurrentLeaderNickname),
			Avatar:   row.CurrentLeaderAvatar,
		}
	}

	return Auction{
		ID:              row.ID,
		Title:           row.Title,
		Description:     row.Description,
		CoverURL:        row.CoverURL,
		Images:          images,
		StreamURL:       row.StreamURL,
		StartPrice:      row.StartPrice,
		PriceStep:       row.PriceStep,
		CeilingPrice:    row.CeilingPrice,
		CurrentPrice:    row.CurrentPrice,
		CurrentLeader:   leader,
		StartTime:       formatTime(row.StartTime, location),
		EndTime:         formatTime(row.EndTime, location),
		OriginalEndTime: formatTime(row.OriginalEndTime, location),
		ExtendSeconds:   row.ExtendSeconds,
		ExtendThreshold: row.ExtendThreshold,
		Status:          row.Status,
		Version:         row.Version,
		ViewerCount:     row.ViewerCount,
		BidCount:        row.BidCount,
		Seller: UserLite{
			ID:       row.SellerID,
			Nickname: row.SellerNickname,
			Avatar:   row.SellerAvatar,
		},
		CreatedAt: formatTime(row.CreatedAt, location),
		UpdatedAt: formatTime(row.UpdatedAt, location),
	}
}

func formatTime(value time.Time, location *time.Location) string {
	return value.In(location).Format(time.RFC3339Nano)
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
