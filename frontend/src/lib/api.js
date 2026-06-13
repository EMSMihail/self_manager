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

export async function updateNoteStatusInBackend(id, status) {
    const res = await fetch('/api/notes', {
        method: 'PUT',
        body: JSON.stringify({ id, status }),
        headers: { 'Content-Type': 'application/json' }
    });
    return res.ok;
}