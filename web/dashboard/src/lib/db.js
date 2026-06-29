import { Pool } from 'pg';

// We default to the same local Postgres URL we used for the Go backend
const pool = new Pool({
  connectionString: process.env.DATABASE_URL || 'postgres://user:pass@localhost:5432/agentsandbox?sslmode=disable',
});

export async function query(text, params) {
  const start = Date.now();
  const res = await pool.query(text, params);
  const duration = Date.now() - start;
  console.log('Executed query', { text, duration, rows: res.rowCount });
  return res;
}
