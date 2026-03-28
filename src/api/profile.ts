export async function saveProfile(data: { html?: string; css?: string }): Promise<{ html: string | null; css: string | null }> {
  const res = await fetch('/api/v1/user/@me/profile', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => null);
    throw new Error(err?.error || 'Failed to save profile');
  }
  return res.json();
}

export async function resetProfile(): Promise<void> {
  const res = await fetch('/api/v1/user/@me/profile/reset', {
    method: 'POST',
  });

  if (!res.ok) {
    const err = await res.json().catch(() => null);
    throw new Error(err?.error || 'Failed to reset profile');
  }
}
