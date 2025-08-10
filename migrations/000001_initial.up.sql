CREATE TABLE players (
    player_id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_name TEXT UNIQUE NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1
);


CREATE TABLE player_hand (
    player_id INTEGER NOT NULL,
    card_id INTEGER NOT NULL,
    card_type INTEGER NOT NULL,
    is_real BOOLEAN NOT NULL,
    PRIMARY KEY (player_id, card_id, card_type, is_real)
);

CREATE UNIQUE INDEX idx_player_card_unique_card ON player_hand (card_id, card_type, is_real);

CREATE TABLE current_player (
    current_player_id INTEGER
)