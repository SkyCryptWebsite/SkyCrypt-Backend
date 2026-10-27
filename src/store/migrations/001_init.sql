-- Light rows: small, kept when blobs are evicted, and the source for leaderboards.

CREATE TABLE players (
	uuid                uuid PRIMARY KEY,
	name_seen_at        timestamptz,
	profiles_fetched_at timestamptz,
	player_fetched_at   timestamptz,
	touched_at          timestamptz NOT NULL DEFAULT now(),
	selected_profile    uuid,
	username            text,
	-- The player's own profile list in Hypixel's order. NULL until their profiles were fetched once.
	profile_ids         uuid[],
	-- /v2/player trimmed to what SkyCrypt reads, zstd.
	player_data         bytea
) WITH (fillfactor = 90);
ALTER TABLE players ALTER COLUMN player_data SET STORAGE EXTERNAL;
CREATE INDEX players_username ON players (lower(username));

CREATE TABLE profiles (
	profile_id uuid PRIMARY KEY,
	seen_at    timestamptz NOT NULL,
	deleted_at timestamptz,
	-- 0 normal, 1 ironman, 2 island (stranded), 3 bingo
	game_mode  smallint NOT NULL DEFAULT 0
) WITH (fillfactor = 90);
CREATE INDEX profiles_deleted_at ON profiles (deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE profile_members (
	profile_id  uuid NOT NULL REFERENCES profiles ON DELETE CASCADE,
	player_uuid uuid NOT NULL,
	-- Compact id for leaderboard entries.
	id          bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
	removed_at  timestamptz,
	-- Each member names the profile after their own fruit, so the name is stored per member.
	cute_name   text,
	PRIMARY KEY (profile_id, player_uuid)
) WITH (fillfactor = 90);
CREATE INDEX profile_members_player ON profile_members (player_uuid);

-- Every stat has a value and the time it was last seen. A player who hides a category from the API keeps the old
-- value, and its time shows how old it is.
CREATE TABLE member_stats (
	member_id      bigint PRIMARY KEY REFERENCES profile_members (id) ON DELETE CASCADE,
	seen_at        timestamptz NOT NULL,
	skyblock_xp_at timestamptz,
	skyblock_xp    integer
) WITH (fillfactor = 90);

-- Heavy rows: compressed bodies, evicted least recently used first.

CREATE TABLE blobs (
	profile_id  uuid NOT NULL,
	-- 1 profile with all members, 2 museum, 3 garden
	kind        smallint NOT NULL,
	-- 0 plain zstd, otherwise the codec_dicts id used
	codec       integer NOT NULL,
	hash        bigint NOT NULL,
	fetched_at  timestamptz NOT NULL,
	-- Changes at most once a day, so the LRU index barely churns.
	accessed_on date NOT NULL DEFAULT current_date,
	bytes       integer NOT NULL,
	-- Empty means Hypixel has nothing (a profile without a garden).
	data        bytea NOT NULL,
	PRIMARY KEY (profile_id, kind)
);
ALTER TABLE blobs ALTER COLUMN data SET STORAGE EXTERNAL;
ALTER TABLE blobs SET (autovacuum_vacuum_scale_factor = 0.02, toast.autovacuum_vacuum_scale_factor = 0.02);
CREATE INDEX blobs_accessed_on ON blobs (accessed_on);

-- zstd dictionaries, trained from stored blobs. Never deleted while a blob may use them.
CREATE TABLE codec_dicts (
	id         integer PRIMARY KEY,
	kind       smallint NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	data       bytea NOT NULL
);

CREATE TABLE resources (
	name       text PRIMARY KEY,
	fetched_at timestamptz NOT NULL,
	data       bytea NOT NULL
);
ALTER TABLE resources ALTER COLUMN data SET STORAGE EXTERNAL;

-- The old key/value table, fallback_cache, is imported and then dropped by import.go.
