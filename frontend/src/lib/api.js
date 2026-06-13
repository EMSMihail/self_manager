// frontend/src/lib/api.js
export async function fetchNotesFromBackend() {
    const res = await fetch('/api/notes');
    return await res.json();
}

export async function sendNoteToBackend(note) {
    const res = await fetch('/api/notes', {
        method: 'POST',
        body: JSON.stringify(note),
        headers: { 'Content-Type': 'application/json' }
    });
    return res.ok;
}

export async function updateNoteInBackend(note) {
    const res = await fetch('/api/notes', {
        method: 'PUT',
        body: JSON.stringify({ 
            id: note.id, 
            content: note.content, 
            deadline: note.deadline, 
            status: note.status,
            notified: note.notified ? true : false // Приведение к boolean для Go
        }),
        headers: { 'Content-Type': 'application/json' }
    });
    return res.ok;
}