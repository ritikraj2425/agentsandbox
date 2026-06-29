import { NextResponse } from 'next/server';
import { query } from '@/lib/db';
import { cookies } from 'next/headers';
import crypto from 'crypto';

function requireAuth() {
  const token = cookies().get('session_token')?.value;
  return token;
}

export async function GET() {
  const userId = requireAuth();
  if (!userId) return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });

  try {
    const res = await query(
      'SELECT name, created_at, revoked, encode(key_hash::bytea, \'hex\') as hash_preview FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC',
      [userId]
    );
    // We only return a preview of the hash for identification, never the raw key
    return NextResponse.json({ keys: res.rows });
  } catch (err) {
    console.error('Error fetching keys:', err);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}

export async function POST(request) {
  const userId = requireAuth();
  if (!userId) return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });

  try {
    const { name } = await request.json();
    if (!name) {
      return NextResponse.json({ error: 'Name is required' }, { status: 400 });
    }

    // Generate raw key
    const rawKey = `sb_live_${crypto.randomBytes(32).toString('hex')}`;
    
    // Hash key for storage (using sha256 as required by Go backend)
    const hash = crypto.createHash('sha256').update(rawKey).digest('hex');

    await query(
      'INSERT INTO api_keys (key_hash, user_id, name, created_at) VALUES ($1, $2, $3, NOW())',
      [hash, userId, name]
    );

    // Return the RAW key ONLY ONCE
    return NextResponse.json({ key: rawKey, name });
  } catch (err) {
    console.error('Error creating key:', err);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}
