import { NextResponse } from 'next/server';
import { query } from '@/lib/db';
import { cookies } from 'next/headers';

export async function GET() {
  const token = cookies().get('session_token')?.value;
  if (!token) {
    return NextResponse.json({ authenticated: false }, { status: 401 });
  }

  try {
    const res = await query('SELECT id, email, created_at FROM users WHERE id = $1', [token]);
    if (res.rows.length === 0) {
      return NextResponse.json({ authenticated: false }, { status: 401 });
    }
    return NextResponse.json({ authenticated: true, user: res.rows[0] });
  } catch (err) {
    console.error('Session error:', err);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}
