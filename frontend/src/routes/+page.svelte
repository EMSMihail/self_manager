<script>
  import { onMount } from 'svelte';
  import { dndzone } from 'svelte-dnd-action';
  import { flip } from 'svelte/animate';
  import { db } from '$lib/db';
  import { fetchNotesFromBackend, sendNoteToBackend, updateNoteInBackend } from '$lib/api';

  const flipDurationMs = 200;
  
  let newNote = "";
  let deadline = "";
  let newPriority = "medium"; // Дефолтный приоритет для новой задачи

  // Переменные для режима редактирования
  let editingId = null;
  let editContent = "";
  let editDeadline = "";

  // Структура Kanban-доски
  let columns = [
    { id: 'todo', name: 'Планы', items: [] },
    { id: 'in_progress', name: 'В процессе', items: [] },
    { id: 'done', name: 'Готово', items: [] }
  ];

  // 1. Умная загрузка и синхронизация
  async function syncAndLoad() {
    try {
      const serverNotes = await fetchNotesFromBackend();
      
      // Получаем локальные заметки, чтобы проверить, нет ли неотправленных изменений
      const localNotes = await db.notes.toArray();
      const localDirtyIds = new Set(localNotes.filter(n => n.isSynced === 0).map(n => n.id));

      // Фильтруем серверные заметки: обновляем в IndexedDB только то, что не редактируется локально прямо сейчас
      const notesToUpsert = serverNotes
        .filter(n => !localDirtyIds.has(n.id))
        .map(n => ({ 
          ...n, 
          status: n.status || 'todo', 
          priority: n.priority || 'medium', // Подтягиваем приоритет с сервера
          isSynced: 1 
        }));

      if (notesToUpsert.length > 0) {
        // bulkPut автоматически обновит существующие id и добавит новые
        await db.notes.bulkPut(notesToUpsert);
      }
      
      // Синхронизируем удаления: если на сервере заметки нет, а у нас она числится синхронизированной — удаляем локально
      const serverIds = new Set(serverNotes.map(n => n.id));
      const idsToDelete = localNotes
        .filter(n => n.isSynced === 1 && !serverIds.has(n.id))
        .map(n => n.id);
        
      if (idsToDelete.length > 0) {
        await db.notes.bulkDelete(idsToDelete);
      }

    } catch (e) {
      console.warn("Бэкенд недоступен, режим автосинхронизации приостановлен", e);
    }
    
    // Перерисовываем колонки на экране
    await loadFromLocal();
  }

  // Чтение данных из IndexedDB (Dexie) и распределение по колонкам
  async function loadFromLocal() {
    const allNotes = await db.notes.orderBy('created_at').reverse().toArray();
    
    columns[0].items = allNotes.filter(n => n.status === 'todo');
    columns[1].items = allNotes.filter(n => n.status === 'in_progress');
    columns[2].items = allNotes.filter(n => n.status === 'done');
    columns = [...columns]; // Принудительное обновление реактивности Svelte
  }

  // 2. Создание новой заметки
  async function addNote() {
    if (!newNote) return;

    const deadlineISO = deadline ? new Date(deadline).toISOString() : null;
    const note = {
      content: newNote,
      deadline: deadlineISO,
      created_at: new Date().toISOString(),
      status: 'todo',
      priority: newPriority, // Сохраняем выбранный приоритет в локальную БД
      isSynced: 0
    };

    const id = await db.notes.add(note);
    await loadFromLocal();

    // Сброс формы в дефолтное состояние
    newNote = "";
    deadline = "";
    newPriority = "medium";

    const success = await sendNoteToBackend({ content: note.content, deadline: note.deadline, priority: note.priority });
    if (success) {
      await db.notes.update(id, { isSynced: 1 });
      await loadFromLocal();
    }
  }

  // 3. Универсальное сохранение изменений (текст и/или дедлайн)
  async function saveEdit(item) {
    const updatedDeadline = editDeadline ? new Date(editDeadline).toISOString() : null;
    
    // Если дедлайн изменился, сбрасываем notified в 0, иначе оставляем старый статус
    const isDeadlineChanged = item.deadline !== updatedDeadline;
    const nextNotifiedStatus = isDeadlineChanged ? 0 : item.notified;

    const updatedNote = {
      ...item,
      content: editContent,
      deadline: updatedDeadline,
      notified: nextNotifiedStatus,
      isSynced: 0
    };

    // Обновляем локально в Dexie
    await db.notes.update(item.id, { content: editContent, deadline: updatedDeadline, notified: nextNotifiedStatus, isSynced: 0 });
    editingId = null;
    await loadFromLocal();

    // Отправляем на бэкенд
    const success = await updateNoteInBackend(updatedNote);
    if (success) {
      await db.notes.update(item.id, { isSynced: 1 });
      await loadFromLocal();
    }
  }

  async function triggerReminderAgain(item) {
    // Форсированно ставим notified в 0 локально
    await db.notes.update(item.id, { notified: 0, isSynced: 0 });
    await loadFromLocal();

    const updatedNote = { ...item, notified: 0 };
    
    // Синхронизируем с сервером
    const success = await updateNoteInBackend(updatedNote);
    if (success) {
      await db.notes.update(item.id, { isSynced: 1 });
      await loadFromLocal();
    }
  }

  // 4. Удаление заметки
  async function deleteNote(id) {
    await db.notes.delete(id);
    await loadFromLocal();
    try {
      await fetch(`/api/notes?id=${id}`, { method: 'DELETE' });
    } catch (e) {
      console.error("Не удалось удалить заметку на сервере:", e);
    }
  }

  // Перевод карточки в режим редактирования
  function startEdit(item) {
    editingId = item.id;
    editContent = item.content;
    
    if (item.deadline) {
      // Преобразуем UTC строку в локальный формат для корректного отображения в input datetime-local
      const d = new Date(item.deadline);
      const tzoffset = d.getTimezoneOffset() * 60000;
      editDeadline = new Date(d.getTime() - tzoffset).toISOString().slice(0, 16);
    } else {
      editDeadline = "";
    }
  }

  // --- ОБРАБОТЧИКИ DRAG AND DROP ---
  function handleConsider(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];
  }

  async function handleFinalize(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];

    for (const item of e.detail.items) {
      if (item.status !== columnId) {
        item.status = columnId;
        await db.notes.update(item.id, { status: columnId, isSynced: 0 });
        
        const success = await updateNoteInBackend(item);
        if (success) {
          await db.notes.update(item.id, { isSynced: 1 });
        }
      }
    }
    await loadFromLocal();
  }

  onMount(() => {
    // Первая загрузка при открытии страницы
    syncAndLoad();

    // Запускаем фоновый опрос сервера каждые 5 секунд
    const interval = setInterval(syncAndLoad, 5000);

    // Функция очистки (вызовется при уничтожении компонента Svelte)
    return () => {
      clearInterval(interval);
    };
  });

  // Хелпер форматирования даты для вывода в карточке
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
    
    <select bind:value={newPriority} class="priority-select">
      <option value="low">🟢 Низкий</option>
      <option value="medium">🟡 Средний</option>
      <option value="high">🔴 Высокий</option>
    </select>

    <button on:click={addNote}>Добавить</button>
  </div>

  <div class="board">
    {#each columns as column (column.id)}
      <div class="column">
        <h2>{column.name}</h2>
        <div class="drop-zone" 
             use:dndzone={{items: column.items, flipDurationMs, dropTargetStyle: {}}} 
             on:consider={(e) => handleConsider(column.id, e)} 
             on:finalize={(e) => handleFinalize(column.id, e)}>
          
          {#each column.items as item (item.id)}
            <div class="card priority-{item.priority || 'medium'}" animate:flip={{duration: flipDurationMs}}>
              <div class="card-content">
                {#if editingId === item.id}
                  <input bind:value={editContent} class="edit-input" />
                  <input type="datetime-local" bind:value={editDeadline} class="edit-input" />
                  <div class="edit-actions">
                    <button on:click={() => saveEdit(item)}>💾</button>
                    <button on:click={() => editingId = null}>❌</button>
                  </div>
                {:else}
                  <div on:click={() => startEdit(item)} class="text-clickable" role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && startEdit(item)}>
                    <span class="content-text">{item.content} {item.isSynced === 0 ? '⏳' : ''}</span>
                    {#if item.deadline}
                      <small class="deadline">Дедлайн: {formatDate(item.deadline)}</small>
                    {/if}
                  </div>
                {/if}
              </div>
              
              {#if editingId !== item.id}
                <div class="card-controls">
                  {#if item.deadline}
                    <button class="action-btn" on:click={() => triggerReminderAgain(item)} title="Напомнить еще раз">🔔</button>
                  {/if}
                  <button class="delete-btn" on:click={() => deleteNote(item.id)}>✕</button>
                </div>
              {/if}
            </div>
          {/each}

        </div>
      </div>
    {/each}
  </div>
</main>

<style>
  main { max-width: 1100px; margin: 2rem auto; font-family: system-ui, -apple-system, sans-serif; padding: 0 20px; }
  .input-group { display: flex; gap: 10px; margin-bottom: 30px; }
  input { padding: 10px; flex-grow: 1; border: 1px solid #ccc; border-radius: 6px; font-size: 0.95rem; }
  button { padding: 10px 18px; cursor: pointer; background: #222; color: white; border: none; border-radius: 6px; font-weight: 500; }
  button:hover { background: #444; }
  
  .board { display: flex; gap: 20px; align-items: flex-start; }
  .column { 
    flex: 1; 
    background: #f1f2f4; 
    border-radius: 10px; 
    padding: 12px; 
    min-height: 500px;
    box-shadow: inset 0 0 4px rgba(0,0,0,0.05);
  }
  h2 { font-size: 1rem; font-weight: 600; text-align: center; margin-bottom: 15px; color: #44546f; text-transform: uppercase; letter-spacing: 0.5px; }
  
  .drop-zone { min-height: 450px; height: 100%; }
  
  .card { 
    background: white; 
    padding: 14px; 
    margin-bottom: 10px; 
    border-radius: 8px; 
    box-shadow: 0 1px 3px rgba(0,0,0,0.1), 0 1px 2px rgba(0,0,0,0.06);
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    cursor: grab;
    transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
    /* Базовая левая граница для цветовой маркировки */
    border-left: 5px solid #dfe1e6; 
  }
  .card:active { cursor: grabbing; }

  /* Цветовые маркеры приоритетов */
  .card.priority-high { border-left-color: #eb5757; }   /* Красный */
  .card.priority-medium { border-left-color: #f2c94c; } /* Желтый */
  .card.priority-low { border-left-color: #27ae60; }    /* Зеленый */

  .card-content { display: flex; flex-direction: column; gap: 5px; width: 100%; }
  
  .text-clickable { cursor: pointer; display: flex; flex-direction: column; gap: 6px; width: 100%; }
  .content-text { font-size: 0.95rem; color: #172b4d; word-break: break-word; }
  
  .edit-input { padding: 8px; margin-bottom: 8px; font-size: 0.9rem; width: 95%; border: 1px solid #0052cc; box-shadow: 0 0 0 1px #0052cc; }
  .edit-actions { display: flex; gap: 8px; }
  .edit-actions button { padding: 6px 12px; background: #f1f2f4; color: #172b4d; font-size: 0.9rem; }
  .edit-actions button:hover { background: #e2e4e9; }

  .deadline { color: #ae2e24; font-size: 0.8rem; font-weight: 500; background: #ffebe6; padding: 2px 6px; border-radius: 4px; width: fit-content; }
  .delete-btn { background: transparent; color: #6b778c; padding: 2px 6px; margin-left: 8px; font-size: 1rem; border-radius: 4px; }
  .delete-btn:hover { color: #ae2e24; background: #ffebe6; }

  .card-controls { display: flex; align-items: center; gap: 4px; }
  .action-btn { background: transparent; color: #6b778c; padding: 4px 6px; font-size: 0.95rem; border-radius: 4px; }
  .action-btn:hover { background: #e2e4e9; color: #172b4d; }
  .delete-btn { background: transparent; color: #6b778c; padding: 4px 8px; font-size: 1rem; border-radius: 4px; margin-left: 0; }

  /* Стилизация селекта в форме */
  .priority-select {
    padding: 10px;
    border: 1px solid #ccc;
    border-radius: 6px;
    background: white;
    font-size: 0.95rem;
    cursor: pointer;
    font-family: inherit;
  }
  .priority-select:focus {
    outline: none;
    border-color: #222;
  }
</style>