import { NextResponse } from 'next/server';
import { query } from '@/lib/db';
import crypto from 'crypto';
import { cookies } from 'next/headers';

export async function POST(request) {
  try {
    const { provider, email } = await request.json();

    if (!email) {
      return NextResponse.json({ error: 'Email is required' }, { status: 400 });
    }

    // Upsert user in Postgres
    let userId;
    const res = await query('SELECT id FROM users WHERE email = $1', [email]);
    if (res.rows.length === 0) {
      userId = `usr_${crypto.randomBytes(16).toString('hex')}`;
      await query(
        'INSERT INTO users (id, email, created_at) VALUES ($1, $2, NOW())',
        [userId, email]
      );
    } else {
      userId = res.rows[0].id;
    }

    // Set a mock session cookie
    cookies().set('session_token', userId, {
      httpOnly: true,
      secure: process.env.NODE_ENV === 'production',
      maxAge: 60 * 60 * 24 * 7, // 1 week
      path: '/',
    });

    return NextResponse.json({ success: true, userId, email });
  } catch (err) {
    console.error('Login error:', err);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}

export async function DELETE() {
  cookies().delete('session_token');
  return NextResponse.json({ success: true });
}
