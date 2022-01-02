CREATE DATABASE IF NOT EXISTS 'basic_service';

CREATE TABLE IF NOT EXISTS leads (
    lead_id serial PRIMARY KEY,
    user_id INT NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    phone VARCHAR(255) NOT NULL,
    notes TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);