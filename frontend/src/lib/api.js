// frontend/src/lib/api.js
export async function fetchNotesFromBackend() {
    const res = await fetch('/api/notes');
    return await res.json();
}

// export async function sendNoteToBackend(note) {
//     const res = await fetch('/api/notes', {
//         method: 'POST',
//         body: JSON.stringify(note),
//         headers: { 'Content-Type': 'application/json' }
//     });
//     return res.ok;
// }
export async function sendNoteToBackend({ content, deadline, priority = 'medium' }) {
    const res = await fetch('/api/notes', {
        method: 'POST',
        body: JSON.stringify({ content, deadline, priority }),
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
            notified: note.notified ? true : false,
            priority: note.priority || 'medium'
        }),
        headers: { 'Content-Type': 'application/json' }
    });
    return res.ok;
}