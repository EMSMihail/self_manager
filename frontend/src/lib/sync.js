import { db } from './db';

export async function syncNotes() {
    // Находим все записи, которые еще не ушли на сервер
    const unsynced = await db.notes.where({ isSynced: 0 }).toArray();

    for (const note of unsynced) {
        const res = await fetch('/api/notes', {
            method: 'POST',
            body: JSON.stringify({ content: note.content, deadline: note.deadline }),
            headers: { 'Content-Type': 'application/json' }
        });

        if (res.ok) {
            // Помечаем как синхронизированную, чтобы больше не отправлять
            await db.notes.update(note.id, { isSynced: 1 });
        }
    }
}