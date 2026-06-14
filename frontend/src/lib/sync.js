import { db } from './db';

export async function syncNotes() {
    const unsynced = await db.notes.where({ isSynced: 0 }).toArray();

    for (const note of unsynced) {
        const res = await fetch('/api/notes', {
            method: 'POST',
            body: JSON.stringify({ 
                content: note.content, 
                description: note.description || '', 
                deadline: note.deadline || '',
                priority: note.priority || 'low'
            }),
            headers: { 'Content-Type': 'application/json' }
        });

        if (res.ok) {
            const data = await res.json();
            await db.notes.delete(note.id);
            await db.notes.put({
                ...note,
                id: data.id,
                isSynced: 1
            });
        }
    }
}