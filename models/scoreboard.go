package models

type (
	Score struct {
		PlayerID int
		Points   int
	}

	UpdatedScore struct {
		PlayerID    int    `json:"player_id"`
		RoundPoints int    `json:"round_points"`
		OldPoints   int    `json:"old_points"`
		NewPoints   int    `json:"new_points"`
		Hand        []Card `json:"cards"`
	}
)
