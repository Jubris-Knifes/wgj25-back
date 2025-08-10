package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"

	"github.com/Jubris-Knifes/wgj25-back/config"
	"github.com/Jubris-Knifes/wgj25-back/models"
	"github.com/georgysavva/scany/sqlscan"
)

type Repository struct {
	db  *sql.DB
	log *slog.Logger
}

func rollback(tx *sql.Tx) {
	if tx != nil {
		tx.Rollback()
	}
}

func New(logger *slog.Logger, db *sql.DB) *Repository {
	return &Repository{
		db:  db,
		log: logger,
	}
}

func (r *Repository) NewPlayer(ctx context.Context, playerName string) (int, error) {
	r.log.DebugContext(ctx, "creating new player", "player_name", playerName)

	tx, err := r.db.BeginTx(ctx, nil)
	defer rollback(tx)
	if err != nil {
		r.log.ErrorContext(ctx, "failed to begin transaction", "error", err)
		return 0, err
	}
	const countQuery = `
		SELECT COUNT(*) from players 
		WHERE is_active = TRUE AND player_name <> ?
	`
	var count int
	if err := sqlscan.Get(ctx, tx, &count, countQuery, playerName); err != nil {
		r.log.ErrorContext(ctx, "failed to count players", "error", err)
		return 0, err
	}

	if count >= config.Get().MaxPlayers {
		r.log.WarnContext(ctx, "player count too high", "count", count, "max", config.Get().MaxPlayers)
		return 0, ErrPlayerCountTooHigh
	}

	const insertQuery = `
		INSERT INTO players (player_name)
		VALUES (?) 
		ON CONFLICT(player_name) DO UPDATE SET is_active = TRUE
		RETURNING player_id
	`
	var playerID int
	if err := sqlscan.Get(ctx, tx, &playerID, insertQuery, playerName); err != nil {
		r.log.ErrorContext(ctx, "failed to insert player", "error", err)
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		r.log.ErrorContext(ctx, "failed to commit transaction", "error", err)
		return 0, err
	}

	r.log.DebugContext(ctx, "new player created", "player_id", playerID, "player_name", playerName)

	return playerID, nil
}

func (r *Repository) ClosePlayer(ctx context.Context, playerID int) error {
	r.log.DebugContext(ctx, "closing player", "id", playerID)

	const query = `
		UPDATE players SET is_active = FALSE WHERE player_id = ?
	`
	_, err := r.db.ExecContext(ctx, query, playerID)
	if err != nil {
		r.log.ErrorContext(ctx, "failed to close player", "error", err)
		return err
	}

	return nil
}

func (r *Repository) GetActivePlayerCount(ctx context.Context) (int, error) {
	r.log.DebugContext(ctx, "getting active player count")

	const query = `
		SELECT COUNT(*) FROM players WHERE is_active = TRUE
	`
	var count int
	if err := sqlscan.Get(ctx, r.db, &count, query); err != nil {
		r.log.ErrorContext(ctx, "failed to get active player count", "error", err)
		return 0, err
	}

	r.log.DebugContext(ctx, "active player count retrieved", "count", count)

	return count, nil
}

func (r *Repository) GetActivePlayerIDs(ctx context.Context) ([]int, error) {
	r.log.DebugContext(ctx, "getting active player IDs")

	const query = `
		SELECT player_id
		FROM players
		WHERE is_active = TRUE
		ORDER BY player_id
	`
	var playerIDs []int
	if err := sqlscan.Select(ctx, r.db, &playerIDs, query); err != nil {
		r.log.ErrorContext(ctx, "failed to get active player IDs", "error", err)
		return nil, err
	}

	r.log.DebugContext(ctx, "active player IDs retrieved", "player_ids", playerIDs)

	return playerIDs, nil
}

func (r *Repository) DropPlayerHands(ctx context.Context) error {
	const query = `--sql
		DELETE FROM player_hands
	`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		r.log.ErrorContext(ctx, "failed to drop player hands", "error", err)
		return err
	}

	return nil
}

func (r *Repository) SetPlayerHand(ctx context.Context, playerID int, cards []models.Card) error {
	r.log.DebugContext(ctx, "setting player hand", "player_id", playerID, "cards", cards)

	var insertQueryBuilder strings.Builder
	insertQueryBuilder.WriteString(`
		INSERT INTO player_hand (player_id, card_id, card_type, is_real) VALUES
	
	`)

	var args = make([]any, 0, len(cards)*4)
	for i, card := range cards {
		insertQueryBuilder.WriteString("(?,?,?,?)")
		if i < len(cards)-1 {
			insertQueryBuilder.WriteString(", ")
		}

		args = append(args, playerID, card.ID, card.Type, card.IsReal)
	}

	r.log.DebugContext(ctx, "inserting player hand", "query", insertQueryBuilder.String(), "args", args)

	if _, err := r.db.ExecContext(ctx, insertQueryBuilder.String(), args...); err != nil {
		r.log.ErrorContext(ctx, "failed to insert player hand", "error", err)
		return err
	}

	r.log.DebugContext(ctx, "player hand set successfully", "player_id", playerID)

	return nil
}

func (r *Repository) GetPlayerHand(ctx context.Context, playerID int) ([]models.Card, error) {
	r.log.DebugContext(ctx, "getting player hand", "player_id", playerID)

	const query = `
		SELECT card_id, card_type, is_real
		FROM player_hand
		WHERE player_id = ?
	`
	var cards []models.Card
	if err := sqlscan.Select(ctx, r.db, &cards, query, playerID); err != nil {
		r.log.ErrorContext(ctx, "failed to get player hand", "error", err)
		return nil, err
	}

	r.log.DebugContext(ctx, "player hand retrieved", "player_id", playerID, "cards", cards)

	return cards, nil
}
func (r *Repository) GetCurrentPlayerID(ctx context.Context) (int, error) {
	r.log.DebugContext(ctx, "getting current player ID")

	const query = `
		SELECT current_player_id FROM current_player
	`
	var playerID int
	if err := sqlscan.Get(ctx, r.db, &playerID, query); err != nil {
		r.log.ErrorContext(ctx, "failed to get current player ID", "error", err)
		return 0, err
	}

	r.log.DebugContext(ctx, "current player ID retrieved", "player_id", playerID)

	return playerID, nil
}

func (r *Repository) SetCurrentPlayerID(ctx context.Context, playerID int) error {
	r.log.DebugContext(ctx, "setting current player ID", "player_id", playerID)

	tx, err := r.db.BeginTx(ctx, nil)
	defer rollback(tx)
	if err != nil {
		r.log.ErrorContext(ctx, "failed to begin transaction", "error", err)
		return err
	}

	const deleteQuery = `--sql
		DELETE FROM current_player
	`

	if _, err := tx.ExecContext(ctx, deleteQuery); err != nil {
		r.log.ErrorContext(ctx, "failed to delete current player", "error", err)
		return err
	}

	const insertQuery = `--sql
		INSERT INTO current_player (current_player_id) VALUES (?)
	`

	if _, err := tx.ExecContext(ctx, insertQuery, playerID); err != nil {
		r.log.ErrorContext(ctx, "failed to insert current player", "error", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.log.ErrorContext(ctx, "failed to commit transaction", "error", err)
		return err
	}

	r.log.DebugContext(ctx, "current player ID set successfully", "player_id", playerID)

	return nil
}
