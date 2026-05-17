CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  email varchar(255) NOT NULL UNIQUE,
  name varchar(120) NOT NULL,
  password_hash varchar(255) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
