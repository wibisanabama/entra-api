SELECT 'CREATE DATABASE entra_auth' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'entra_auth')\gexec
SELECT 'CREATE DATABASE entra_event' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'entra_event')\gexec
SELECT 'CREATE DATABASE entra_ticket' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'entra_ticket')\gexec
SELECT 'CREATE DATABASE entra_payment' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'entra_payment')\gexec
SELECT 'CREATE DATABASE entra_cashless' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'entra_cashless')\gexec;
