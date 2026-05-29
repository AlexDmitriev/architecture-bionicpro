CREATE DATABASE IF NOT EXISTS olap;

CREATE TABLE IF NOT EXISTS olap.crm_customers_kafka
(
    raw String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka:9092',
    kafka_topic_list = 'crm.public.customers',
    kafka_group_name = 'clickhouse-crm-cdc',
    kafka_format = 'JSONAsString',
    kafka_num_consumers = 1,
    kafka_handle_error_mode = 'stream';

CREATE TABLE IF NOT EXISTS olap.crm_customers
(
    id Int32,
    name String,
    email String,
    age Nullable(Float64),
    gender Nullable(String),
    country Nullable(String),
    address Nullable(String),
    phone Nullable(String),
    is_deleted UInt8 DEFAULT 0,
    updated_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (id);

CREATE MATERIALIZED VIEW IF NOT EXISTS olap.mv_crm_customers_from_kafka
TO olap.crm_customers
AS
SELECT
    JSONExtractInt(raw, 'payload', 'id') AS id,
    JSONExtractString(raw, 'payload', 'name') AS name,
    JSONExtractString(raw, 'payload', 'email') AS email,
    CAST(NULL, 'Nullable(Float64)') AS age,
    nullIf(JSONExtractString(raw, 'payload', 'gender'), '') AS gender,
    nullIf(JSONExtractString(raw, 'payload', 'country'), '') AS country,
    nullIf(JSONExtractString(raw, 'payload', 'address'), '') AS address,
    nullIf(JSONExtractString(raw, 'payload', 'phone'), '') AS phone,
    if(JSONExtractString(raw, 'payload', '__op') = 'd' OR JSONExtractString(raw, 'payload', '__deleted') = 'true', 1, 0) AS is_deleted,
    now() AS updated_at
FROM olap.crm_customers_kafka;

CREATE TABLE IF NOT EXISTS olap.mart_user_report
(
    user_id String,
    customer_id Int32,
    customer_name String,
    country String,
    report_period_start Date,
    report_period_end Date,
    sessions_count Int32,
    total_usage_minutes Float64,
    movements_count Int32,
    avg_battery_pct Float64,
    errors_count Int32,
    report_payload String,
    updated_at DateTime
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS olap.mv_mart_user_report_from_crm
TO olap.mart_user_report
AS
SELECT
    email AS user_id,
    id AS customer_id,
    name AS customer_name,
    ifNull(country, '') AS country,
    toDate(now() - toIntervalDay(1)) AS report_period_start,
    toDate(now()) AS report_period_end,
    0 AS sessions_count,
    0.0 AS total_usage_minutes,
    0 AS movements_count,
    0.0 AS avg_battery_pct,
    0 AS errors_count,
    toJSONString(
        map(
            'user_id', email,
            'customer_name', name,
            'country', ifNull(country, ''),
            'report_period_start', toString(toDate(now() - toIntervalDay(1))),
            'report_period_end', toString(toDate(now())),
            'sessions_count', '0',
            'total_usage_minutes', '0',
            'movements_count', '0',
            'avg_battery_pct', '0',
            'errors_count', '0'
        )
    ) AS report_payload,
    now() AS updated_at
FROM olap.crm_customers
WHERE is_deleted = 0
  AND email != '';
