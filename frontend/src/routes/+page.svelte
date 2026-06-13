<script>
  import { onMount } from 'svelte';
  import { db } from '$lib/db';
  import { fetchNotesFromBackend, sendNoteToBackend } from '$lib/api';

  let notes = [];
  let newNote = "";
  let deadline = "";

  async function syncAndLoad() {
    try {
      const serverNotes = await fetchNotesFromBackend();
      await db.notes.clear();
      await db.notes.bulkAdd(serverNotes.map(n => ({ ...n, isSynced: 1 })));
    } catch (e) {
      console.warn("Бэкенд недоступен, работаем оффлайн", e);
    }
    await loadFromLocal();
  }

  async function loadFromLocal() {
    notes = await db.notes.orderBy('created_at').reverse().toArray();
  }

  async function addNote() {
    if (!newNote) return;
    const note = {
      content: newNote,
      deadline: deadline || null,
      created_at: new Date().toISOString(),
      isSynced: 0
    };
    const id = await db.notes.add(note);
    await loadFromLocal();

    newNote = "";
    deadline = "";

    const success = await sendNoteToBackend({ content: note.content, deadline: note.deadline });
    if (success) {
      await db.notes.update(id, { isSynced: 1 });
      await loadFromLocal();
    }
  }

  // Восстановленная функция удаления
  async function deleteNote(id) {
    // Удаляем из локальной базы
    await db.notes.delete(id);
    await loadFromLocal();

    // Удаляем на бэкенде
    try {
      await fetch(`/api/notes?id=${id}`, { method: 'DELETE' });
    } catch (e) {
      console.error("Ошибка удаления на сервере", e);
    }
  }

  onMount(syncAndLoad);

  function formatDate(dateStr) {
    const date = new Date(dateStr);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }
</script>

<main>
  <h1>Мои заметки</h1>

  <div class="input-group">
    <input bind:value={newNote} placeholder="Что нужно сделать?" />
    <input bind:value={deadline} type="datetime-local" />
    <button on:click={addNote}>Добавить</button>
  </div>

  <ul class="notes-list">
    {#each notes as note}
      <li class="note-item">
        <span>{note.content} {note.isSynced === 0 ? '⏳' : ''}</span>
        <div class="actions">
            <small>{formatDate(note.created_at)}</small>
            <button on:click={() => deleteNote(note.id)}>✕</button>
        </div>
      </li>
    {/each}
  </ul>
</main>

<style>
  main { max-width: 500px; margin: 2rem auto; font-family: sans-serif; }
  .input-group { display: flex; gap: 10px; margin-bottom: 20px; }
  .notes-list { list-style: none; padding: 0; }
  .note-item { 
    display: flex; 
    justify-content: space-between; 
    align-items: center;
    padding: 10px; 
    border-bottom: 1px solid #eee; 
  }
  .actions { display: flex; align-items: center; gap: 10px; }
  input { padding: 8px; flex-grow: 1; }
  button { padding: 8px; cursor: pointer; }
</style>