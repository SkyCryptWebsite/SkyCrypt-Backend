-- Skull texture strings, stored once. Blobs keep only their ids (see textures.go).
CREATE TABLE texture_strings (
	id   integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	-- sha256 of the string, cut to 16 bytes.
	hash bytea NOT NULL UNIQUE,
	-- zstd
	data bytea NOT NULL
);

-- 0 as written by ingest, 1 once the archive worker has recompressed it (see archive.go).
ALTER TABLE blobs ADD COLUMN level smallint NOT NULL DEFAULT 0;

-- 0 plain zstd, otherwise the codec_dicts id used for player_data.
ALTER TABLE players ADD COLUMN player_codec integer NOT NULL DEFAULT 0;

-- Pack light rows full. An update now usually moves to another page, which costs a little index churn that vacuum
-- reclaims; at millions of rows the 10% is worth more. Only pages written from now on are affected.
ALTER TABLE players SET (fillfactor = 100);
ALTER TABLE profiles SET (fillfactor = 100);
ALTER TABLE profile_members SET (fillfactor = 100);
ALTER TABLE member_values SET (fillfactor = 100);
ALTER TABLE profile_values SET (fillfactor = 100);
