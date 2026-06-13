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