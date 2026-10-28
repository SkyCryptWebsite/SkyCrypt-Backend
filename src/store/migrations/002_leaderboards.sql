-- Players kept off every leaderboard. Their data is still stored and served.
CREATE TABLE lb_hidden (
	uuid      uuid PRIMARY KEY,
	hidden_at timestamptz NOT NULL DEFAULT now(),
	reason    text NOT NULL DEFAULT ''
);
