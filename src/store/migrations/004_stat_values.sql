-- Leaderboards rank hundreds of values per member (every skill, collection, mob kill and death, floor time, ...).
-- A column or row per value would not fit, so each member and profile keeps all of its values in one packed record,
-- and stat keys are numbered once in stat_keys. Redis holds only the top entries of each board.

-- Replaces the per-stat columns of 003.
DROP TABLE member_stats;

-- Profiles rank on their own boards (coop collections, garden), keyed by this compact id.
ALTER TABLE profiles ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY UNIQUE;

CREATE TABLE stat_keys (
	id  integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	key text NOT NULL UNIQUE
);

-- data: per value, uvarint stat id, zigzag varint value, uvarint seconds between updated_at and seen_at (0 when
-- the value was visible on the last fetch, more when the player has hidden it since).
-- ranked and game_mode say which boards the record is on right now, so leaving, hiding or a mode change can take
-- it off exactly those.
CREATE TABLE member_values (
	member_id bigint PRIMARY KEY REFERENCES profile_members (id) ON DELETE CASCADE,
	seen_at   timestamptz NOT NULL,
	version   smallint NOT NULL,
	game_mode smallint NOT NULL,
	ranked    boolean NOT NULL,
	data      bytea NOT NULL
) WITH (fillfactor = 90);

CREATE TABLE profile_values (
	profile_id uuid PRIMARY KEY REFERENCES profiles ON DELETE CASCADE,
	seen_at    timestamptz NOT NULL,
	version    smallint NOT NULL,
	game_mode  smallint NOT NULL,
	ranked     boolean NOT NULL,
	data       bytea NOT NULL
) WITH (fillfactor = 90);
