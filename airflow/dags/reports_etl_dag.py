from datetime import datetime, timedelta

from airflow import DAG
from airflow.providers.postgres.operators.postgres import PostgresOperator


default_args = {
    "owner": "bionicpro",
    "depends_on_past": False,
    "retries": 1,
    "retry_delay": timedelta(minutes=5),
}


with DAG(
    dag_id="reports_etl_dag",
    default_args=default_args,
    start_date=datetime(2026, 1, 1),
    schedule="0 2 * * *",
    catchup=False,
    tags=["reports", "etl", "olap"],
) as dag:
    extract_crm = PostgresOperator(
        task_id="extract_crm",
        postgres_conn_id="crm_postgres",
        sql="""
        SELECT 1;
        """,
    )

    load_crm_staging = PostgresOperator(
        task_id="load_crm_staging",
        postgres_conn_id="olap_postgres",
        sql="""
        TRUNCATE TABLE olap.stg_crm_customers;

        INSERT INTO olap.stg_crm_customers (customer_id, customer_name, user_id, country, updated_at)
        SELECT id, name, email, country, NOW()
        FROM dblink(
            'host=crm_db port=5432 dbname=crm user=crm password=crm',
            'SELECT id, name, email, country FROM customers'
        ) AS c(id INT, name VARCHAR, email VARCHAR, country VARCHAR);
        """,
    )

    extract_telemetry = PostgresOperator(
        task_id="extract_telemetry",
        postgres_conn_id="telemetry_postgres",
        sql="""
        TRUNCATE TABLE olap.stg_telemetry_agg;

        INSERT INTO olap.stg_telemetry_agg (
            user_id,
            sessions_count,
            total_usage_minutes,
            movements_count,
            avg_battery_pct,
            errors_count,
            report_period_start,
            report_period_end,
            updated_at
        )
        SELECT
            user_id,
            COUNT(*) FILTER (WHERE session_seconds > 0) AS sessions_count,
            ROUND(COALESCE(SUM(session_seconds), 0)::numeric / 60, 2) AS total_usage_minutes,
            COUNT(*) FILTER (WHERE event_type = 'movement') AS movements_count,
            ROUND(AVG(battery_pct), 2) AS avg_battery_pct,
            COUNT(*) FILTER (WHERE is_error) AS errors_count,
            (CURRENT_DATE - INTERVAL '1 day')::date AS report_period_start,
            CURRENT_DATE::date AS report_period_end,
            NOW() AS updated_at
        FROM telemetry_events
        WHERE recorded_at >= (CURRENT_DATE - INTERVAL '1 day')
        GROUP BY user_id;
        """,
    )

    build_datamart = PostgresOperator(
        task_id="build_datamart",
        postgres_conn_id="olap_postgres",
        sql="""
        WITH crm_dedup AS (
            SELECT DISTINCT ON (c.user_id)
                c.user_id,
                c.customer_id,
                c.customer_name,
                c.country
            FROM olap.stg_crm_customers c
            WHERE c.user_id IS NOT NULL AND btrim(c.user_id) <> ''
            ORDER BY c.user_id, c.customer_id DESC
        )
        INSERT INTO olap.mart_user_report (
            user_id,
            customer_id,
            customer_name,
            country,
            report_period_start,
            report_period_end,
            sessions_count,
            total_usage_minutes,
            movements_count,
            avg_battery_pct,
            errors_count,
            report_payload,
            updated_at
        )
        SELECT
            c.user_id,
            c.customer_id,
            c.customer_name,
            c.country,
            COALESCE(t.report_period_start, CURRENT_DATE - INTERVAL '1 day'),
            COALESCE(t.report_period_end, CURRENT_DATE),
            COALESCE(t.sessions_count, 0),
            COALESCE(t.total_usage_minutes, 0),
            COALESCE(t.movements_count, 0),
            COALESCE(t.avg_battery_pct, 0),
            COALESCE(t.errors_count, 0),
            jsonb_build_object(
                'user_id', c.user_id,
                'customer_name', c.customer_name,
                'country', c.country,
                'report_period_start', COALESCE(t.report_period_start, CURRENT_DATE - INTERVAL '1 day'),
                'report_period_end', COALESCE(t.report_period_end, CURRENT_DATE),
                'sessions_count', COALESCE(t.sessions_count, 0),
                'total_usage_minutes', COALESCE(t.total_usage_minutes, 0),
                'movements_count', COALESCE(t.movements_count, 0),
                'avg_battery_pct', COALESCE(t.avg_battery_pct, 0),
                'errors_count', COALESCE(t.errors_count, 0)
            ),
            NOW()
        FROM crm_dedup c
        LEFT JOIN olap.stg_telemetry_agg t ON t.user_id = c.user_id
        ON CONFLICT (user_id) DO UPDATE
        SET
            customer_id = EXCLUDED.customer_id,
            customer_name = EXCLUDED.customer_name,
            country = EXCLUDED.country,
            report_period_start = EXCLUDED.report_period_start,
            report_period_end = EXCLUDED.report_period_end,
            sessions_count = EXCLUDED.sessions_count,
            total_usage_minutes = EXCLUDED.total_usage_minutes,
            movements_count = EXCLUDED.movements_count,
            avg_battery_pct = EXCLUDED.avg_battery_pct,
            errors_count = EXCLUDED.errors_count,
            report_payload = EXCLUDED.report_payload,
            updated_at = NOW();
        """,
    )

    prepare_dblink = PostgresOperator(
        task_id="prepare_dblink",
        postgres_conn_id="olap_postgres",
        sql="""
        CREATE EXTENSION IF NOT EXISTS dblink;
        """,
    )

    extract_crm >> prepare_dblink >> load_crm_staging
    extract_telemetry >> build_datamart
    load_crm_staging >> build_datamart
