<script>
  import { onMount } from 'svelte';
  import { dndzone } from 'svelte-dnd-action';
  import { flip } from 'svelte/animate';
  import { db } from '$lib/db';
  import { fetchNotesFromBackend, sendNoteToBackend, updateNoteStatusInBackend } from '$lib/api';

  const flipDurationMs = 200;
  
  let newNote = "";
  let deadline = "";

  // Структура нашей доски
  let columns = [
    { id: 'todo', name: 'Планы', items: [] },
    { id: 'in_progress', name: 'В процессе', items: [] },
    { id: 'done', name: 'Готово', items: [] }
  ];

  async function syncAndLoad() {
    try {
      const serverNotes = await fetchNotesFromBackend();
      await db.notes.clear();
      // Если у старых записей нет статуса с сервера, ставим 'todo'
      await db.notes.bulkAdd(serverNotes.map(n => ({ ...n, status: n.status || 'todo', isSynced: 1 })));
    } catch (e) {
      console.warn("Бэкенд недоступен", e);
    }
    await loadFromLocal();
  }

  async function loadFromLocal() {
    const allNotes = await db.notes.orderBy('created_at').reverse().toArray();
    
    // Раскладываем заметки по колонкам
    columns[0].items = allNotes.filter(n => n.status === 'todo');
    columns[1].items = allNotes.filter(n => n.status === 'in_progress');
    columns[2].items = allNotes.filter(n => n.status === 'done');
    columns = [...columns]; // Триггерим реактивность
  }

  async function addNote() {
    if (!newNote) return;
    const note = {
      content: newNote,
      deadline: deadline ? new Date(deadline).toISOString() : null,
      created_at: new Date().toISOString(),
      status: 'todo', // Новая задача всегда попадает в "Планы"
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

  async function deleteNote(id) {
    await db.notes.delete(id);
    await loadFromLocal();
    try {
      await fetch(`/api/notes?id=${id}`, { method: 'DELETE' });
    } catch (e) {
      console.error(e);
    }
  }

  // --- ЛОГИКА DRAG AND DROP ---
  function handleConsider(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];
  }

  async function handleFinalize(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];

    // Проверяем, изменился ли статус у какого-либо элемента
    for (const item of e.detail.items) {
      if (item.status !== columnId) {
        item.status = columnId; // Обновляем локально
        await db.notes.update(item.id, { status: columnId }); // В Dexie
        await updateNoteStatusInBackend(item.id, columnId); // На сервер
      }
    }
  }

  onMount(syncAndLoad);

  function formatDate(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }
</script>

<main>
  <h1>Kanban Менеджер</h1>

  <div class="input-group">
    <input bind:value={newNote} placeholder="Что нужно сделать?" />
    <input bind:value={deadline} type="datetime-local" />
    <button on:click={addNote}>Добавить</button>
  </div>

  <div class="board">
    {#each columns as column (column.id)}
      <div class="column">
        <h2>{column.name}</h2>
        <div class="drop-zone" 
             use:dndzone={{items: column.items, flipDurationMs}} 
             on:consider={(e) => handleConsider(column.id, e)} 
             on:finalize={(e) => handleFinalize(column.id, e)}>
          
          {#each column.items as item (item.id)}
            <div class="card" animate:flip={{duration: flipDurationMs}}>
              <div class="card-content">
                <span>{item.content} {item.isSynced === 0 ? '⏳' : ''}</span>
                {#if item.deadline}
                  <small class="deadline">Дедлайн: {formatDate(item.deadline)}</small>
                {/if}
              </div>
              <button class="delete-btn" on:click={() => deleteNote(item.id)}>✕</button>
            </div>
          {/each}

        </div>
      </div>
    {/each}
  </div>
</main>

<style>
  main { max-width: 1000px; margin: 2rem auto; font-family: sans-serif; padding: 0 20px; }
  .input-group { display: flex; gap: 10px; margin-bottom: 30px; }
  input { padding: 10px; flex-grow: 1; border: 1px solid #ccc; border-radius: 4px;}
  button { padding: 10px 15px; cursor: pointer; background: #333; color: white; border: none; border-radius: 4px;}
  
  .board { display: flex; gap: 20px; align-items: flex-start; }
  .column { 
    flex: 1; 
    background: #f4f5f7; 
    border-radius: 8px; 
    padding: 10px; 
    min-height: 400px;
  }
  h2 { font-size: 1.1rem; text-align: center; margin-bottom: 15px; color: #5e6c84; }
  
  .drop-zone { min-height: 350px; }
  
  .card { 
    background: white; 
    padding: 15px; 
    margin-bottom: 10px; 
    border-radius: 4px; 
    box-shadow: 0 1px 3px rgba(0,0,0,0.12);
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    cursor: grab;
  }
  .card-content { display: flex; flex-direction: column; gap: 5px; }
  .deadline { color: #d9534f; font-size: 0.8rem; }
  .delete-btn { background: transparent; color: #999; padding: 0; margin-left: 10px; }
  .delete-btn:hover { color: red; }
</style>