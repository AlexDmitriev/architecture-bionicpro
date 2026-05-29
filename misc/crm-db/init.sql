CREATE TABLE IF NOT EXISTS customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100),
    age NUMERIC,
    gender VARCHAR(10),
    country VARCHAR(100),
    address VARCHAR(255),
    phone VARCHAR(25)
);

COPY customers(id, name, email, age, gender, country, address, phone)
FROM '/docker-entrypoint-initdb.d/crm.csv'
DELIMITER ','
CSV HEADER;

SELECT setval(
    pg_get_serial_sequence('customers', 'id'),
    COALESCE((SELECT MAX(id) FROM customers), 1)
);

INSERT INTO customers (name, email, age, gender, country, address, phone)
SELECT 'Alex Johnson', 'alex@example.com', 31, 'Male', 'Russia', 'Moscow', '+7-900-000-00-01'
WHERE NOT EXISTS (
    SELECT 1 FROM customers WHERE email = 'alex@example.com'
);

INSERT INTO customers (name, email, age, gender, country, address, phone)
SELECT 'John Doe', 'john@example.com', 34, 'Male', 'Russia', 'Saint Petersburg', '+7-900-000-00-02'
WHERE NOT EXISTS (
    SELECT 1 FROM customers WHERE email = 'john@example.com'
);