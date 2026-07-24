CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.events
(
    type      LowCardinality(String),
    user_id   String,
    payload   String DEFAULT '',
    event_ts  DateTime DEFAULT now()
)
ENGINE = MergeTree
ORDER BY (type, event_ts);
