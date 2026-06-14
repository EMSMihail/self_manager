<script>
  import { onMount } from 'svelte';
  import { dndzone } from 'svelte-dnd-action';
  import { flip } from 'svelte/animate';
  import { db } from '$lib/db';
  import { fetchNotesFromBackend, sendNoteToBackend, updateNoteInBackend } from '$lib/api';

  const flipDurationMs = 200;
  let newNote = "";
  let newDescription = "";
  let deadline = "";
  let newPriority = "low";

  // Переменные для режима редактирования
  let editingId = null;
  let editContent = "";
  let editDescription = "";
  let editDeadline = "";
  let editPriority = "low";

  // Модалка для дедлайна при перетаскивании в In Progress
  let showDeadlineModal = false;
  let modalNote = null;
  let modalDeadlineValue = "";

  // Переменные управления сортировкой и фильтрацией
  let filterPriority = "all"; // "all", "low", "medium", "high"
  let filterUrgent = false;   // true / false
  let sortBy = "none";        // "none", "priority", "deadline"

  // Структура Kanban-доски
  let columns = [
    { id: 'todo', name: 'Планы', items: [] },
    { id: 'in_progress', name: 'В процессе', items: [] },
    { id: 'done', name: 'Готово', items: [] }
  ];

  async function syncAndLoad() {
    try {
      const serverNotes = await fetchNotesFromBackend();
      const localNotes = await db.notes.toArray();
      const localDirtyIds = new Set(localNotes.filter(n => n.isSynced === 0).map(n => n.id));
      const notesToUpsert = serverNotes.filter(sn => !localDirtyIds.has(sn.id));
      
      for (const note of notesToUpsert) {
        await db.notes.put({
          id: note.id,
          content: note.content,
          description: note.description || '',
          deadline: note.deadline ? new Date(note.deadline).toISOString() : null,
          notified: note.notified ? 1 : 0,
          status: note.status || 'todo',
          priority: note.priority || 'low',
          created_at: note.created_at,
          isSynced: 1
        });
      }

      const serverIds = new Set(serverNotes.map(sn => sn.id));
      for (const localNote of localNotes) {
        if (localNote.isSynced === 1 && !serverIds.has(localNote.id)) {
          await db.notes.delete(localNote.id);
        }
      }

      await refreshBoardFromIndexedDB();
    } catch (err) {
      console.warn("Работаем в офлайн-режиме:", err);
      await refreshBoardFromIndexedDB();
    }
  }

  async function refreshBoardFromIndexedDB() {
    const allLocal = await db.notes.toArray();
    const weights = { high: 3, medium: 2, low: 1 };

    columns = columns.map(col => {
      let items = allLocal
        .filter(n => n.status === col.id)
        .map(n => ({ ...n, id: String(n.id) })); // dnd-zone требует строковые ID

      // Применяем сортировку, если выбран режим
      if (sortBy === 'priority') {
        items.sort((a, b) => (weights[b.priority] || 0) - (weights[a.priority] || 0));
      } else if (sortBy === 'deadline') {
        items.sort((a, b) => {
          if (!a.deadline) return 1;
          if (!b.deadline) return -1;
          return a.deadline.localeCompare(b.deadline);
        });
      }

      return { ...col, items };
    });
  }

  // Функция проверки видимости карточки (передаем реактивные переменные аргументами)
  function isCardVisible(item, currentPriorityFilter, currentUrgentFilter) {
    if (currentPriorityFilter !== "all" && item.priority !== currentPriorityFilter) return false;
    if (currentUrgentFilter) {
      if (!item.deadline) return false;
      const todayStr = new Date().toISOString().split('T')[0];
      return item.deadline.split('T')[0] <= todayStr; // Проверяем, если дедлайн сегодня или просрочен
    }
    return true;
  }

  onMount(async () => {
    await syncAndLoad();
    setInterval(syncAndLoad, 15000);
  });

  // Добавление новой карточки
  async function addNote() {
    if (!newNote.trim()) return;
    const localId = Date.now();
    const noteData = {
      content: newNote,
      description: newDescription,
      deadline: null,
      status: 'todo',
      notified: 0,
      priority: newPriority,
      created_at: new Date().toISOString(),
      isSynced: 0
    };

    // Быстро отображаем карточку в UI с временным ID
    await db.notes.put({ ...noteData, id: localId });
    newNote = "";
    newDescription = "";
    newPriority = "low";
    await refreshBoardFromIndexedDB();

    // Отправляем на бэкенд
    const result = await sendNoteToBackend({ 
      content: noteData.content, 
      description: noteData.description,
      deadline: "", 
      priority: noteData.priority 
    });

    if (result && result.id) {
      await db.notes.delete(localId);
      await db.notes.put({
        ...noteData,
        id: result.id,
        isSynced: 1
      });
      await syncAndLoad();
    }
  }

  // Быстрая кнопка колокольчика (+1 час дедлайна)
  async function addHourReminder(item) {
    const oneHourLater = new Date(Date.now() + 60 * 60 * 1000).toISOString();
    await db.notes.update(Number(item.id), {
      deadline: oneHourLater,
      notified: 0,
      isSynced: 0
    });
    await refreshBoardFromIndexedDB();

    const updatedNote = await db.notes.get(Number(item.id));
    const success = await updateNoteInBackend(updatedNote);
    if (success) {
      await db.notes.update(Number(item.id), { isSynced: 1 });
    }
  }

  // Хэндлеры для Drag and Drop
  function handleDndConsider(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];
  }

  async function handleDndFinalize(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];

    const triggeredItem = e.detail.info?.id 
      ? e.detail.items.find(i => i.id === e.detail.info.id)
      : null;
    if (triggeredItem) {
      const numericId = Number(triggeredItem.id);
      const originalNote = await db.notes.get(numericId);
      
      if (columnId === 'in_progress' && originalNote.status === 'todo') {
        modalNote = triggeredItem;
        modalDeadlineValue = "";
        showDeadlineModal = true;
        return;
      }

      let updatedDeadline = originalNote.deadline;
      if (columnId === 'done') {
        updatedDeadline = null;
      }

      await db.notes.update(numericId, { 
        status: columnId, 
        deadline: updatedDeadline,
        isSynced: 0 
      });
      await refreshBoardFromIndexedDB();

      const fullNote = await db.notes.get(numericId);
      const success = await updateNoteInBackend(fullNote);
      if (success) {
        await db.notes.update(numericId, { isSynced: 1 });
      }
    }
  }

  // Сохранение дедлайна из модалки
  async function saveModalDeadline() {
    if (!modalNote) return;
    const numericId = Number(modalNote.id);
    const formattedDeadline = modalDeadlineValue ? new Date(modalDeadlineValue).toISOString() : null;
    await db.notes.update(numericId, {
      status: 'in_progress',
      deadline: formattedDeadline,
      notified: 0,
      isSynced: 0
    });
    closeModal();
    await refreshBoardFromIndexedDB();

    const fullNote = await db.notes.get(numericId);
    const success = await updateNoteInBackend(fullNote);
    if (success) {
      await db.notes.update(numericId, { isSynced: 1 });
    }
  }

  // Пропуск дедлайна в модалке
  async function skipModalDeadline() {
    if (!modalNote) return;
    const numericId = Number(modalNote.id);

    await db.notes.update(numericId, {
      status: 'in_progress',
      deadline: null,
      isSynced: 0
    });
    closeModal();
    await refreshBoardFromIndexedDB();

    const fullNote = await db.notes.get(numericId);
    const success = await updateNoteInBackend(fullNote);
    if (success) {
      await db.notes.update(numericId, { isSynced: 1 });
    }
  }

  function closeModal() {
    showDeadlineModal = false;
    modalNote = null;
    modalDeadlineValue = "";
  }

  // Удаление карточки
  async function deleteCard(id) {
    const numericId = Number(id);
    await db.notes.delete(numericId);
    await refreshBoardFromIndexedDB();

    try {
      await fetch(`/api/notes?id=${numericId}`, { method: 'DELETE' });
    } catch (e) {
      console.error("Не удалось удалить на сервере, удалено локально", e);
    }
  }

  // Редактирование текста напрямую
  function startEdit(item) {
    editingId = item.id;
    editContent = item.content;
    editDescription = item.description || "";
    editDeadline = item.deadline ? item.deadline.slice(0, 16) : "";
    editPriority = item.priority || "low";
  }

  async function saveEdit() {
    const numericId = Number(editingId);
    await db.notes.update(numericId, {
      content: editContent,
      description: editDescription,
      deadline: editDeadline ? new Date(editDeadline).toISOString() : null,
      priority: editPriority,
      isSynced: 0
    });
    editingId = null;
    await refreshBoardFromIndexedDB();

    const fullNote = await db.notes.get(numericId);
    const success = await updateNoteInBackend(fullNote);
    if (success) {
      await db.notes.update(numericId, { isSynced: 1 });
    }
  }

  function formatDisplayDate(isoString) {
    if (!isoString) return '';
    const d = new Date(isoString);
    return d.toLocaleString('ru-RU', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }
</script>

<main class="app-container">
  <header class="app-header">
    <div class="logo">🗂 Self<span>Manager</span></div>
    <div class="status-indicator">Синхронизация: Active</div>
  </header>

  <section class="creation-panel">
    <div class="input-wrapper">
      <input 
        type="text" 
        bind:value={newNote} 
        placeholder="Какая задача перед нами стоит?.." 
        on:keydown={(e) => e.key === 'Enter' && addNote()}
      />
      
      <select bind:value={newPriority} class="priority-select {newPriority}">
        <option value="low">🟢 Низкий</option>
        <option value="medium">🟡 Средний</option>
        <option value="high">🔴 Высокий</option>
      </select>

      <button on:click={addNote} class="add-button">Создать</button>
    </div>
    <div class="description-wrapper">
      <textarea 
        bind:value={newDescription} 
        placeholder="Добавить описание к задаче (необязательно)..." 
        rows="2"
        class="creation-description"
      ></textarea>
    </div>
  </section>

  <div class="toolbar">
    <div class="filter-group">
      <label for="priority-filter">Приоритет:</label>
      <select id="priority-filter" bind:value={filterPriority}>
        <option value="all">Все</option>
        <option value="low">🟢 Низкий</option>
        <option value="medium">🟡 Средний</option>
        <option value="high">🔴 Высокий</option>
      </select>
    </div>

    <div class="filter-group">
      <label for="sort-select">Сортировка:</label>
      <select id="sort-select" bind:value={sortBy} on:change={refreshBoardFromIndexedDB}>
        <option value="none">По порядку</option>
        <option value="priority">По приоритету (Важные сверху)</option>
        <option value="deadline">По дедлайну (Срочные сверху)</option>
      </select>
    </div>

    <div class="filter-group checkbox-group">
      <label>
        <input type="checkbox" bind:checked={filterUrgent}>
        🔥 Только горящие (сегодня / просрочено)
      </label>
    </div>
  </div>

  <div class="kanban-board">
    {#each columns as column (column.id)}
      <div class="kanban-column">
        <div class="column-header">
          <h3>{column.name}</h3>
          <span class="badge">{column.items.length}</span>
        </div>
        
        <div 
          class="column-body"
          use:dndzone={{ items: column.items, flipDurationMs }}
          on:consider={(e) => handleDndConsider(column.id, e)}
          on:finalize={(e) => handleDndFinalize(column.id, e)}
        >
          {#each column.items as item (item.id)}
            <div 
              animate:flip={{ duration: flipDurationMs }} 
              class="card priority-{item.priority} {isCardVisible(item, filterPriority, filterUrgent) ? '' : 'hidden-card'}"
            >
              {#if editingId === item.id}
                <div class="edit-mode">
                  <input type="text" bind:value={editContent} class="edit-title-input" placeholder="Название задачи" />
                  <textarea bind:value={editDescription} rows="3" placeholder="Описание задачи..."></textarea>
                  <div class="edit-row">
                    <input type="datetime-local" bind:value={editDeadline} />
                    <select bind:value={editPriority}>
                      <option value="low">Low</option>
                      <option value="medium">Medium</option>
                      <option value="high">High</option>
                    </select>
                  </div>
                  <div class="edit-actions">
                    <button class="btn-save" on:click={saveEdit}>Готово</button>
                    <button class="btn-cancel" on:click={() => editingId = null}>Отмена</button>
                  </div>
                </div>
              {:else}
                <div class="card-layout">
                  <div class="card-main">
                    <h4 class="card-text">{item.content}</h4>
                    {#if item.description}
                      <p class="card-description">{item.description}</p>
                    {/if}
                    {#if item.deadline}
                      <span class="card-deadline">⏰ {formatDisplayDate(item.deadline)}</span>
                    {/if}
                  </div>
                  
                  <div class="card-controls">
                    <button class="action-btn bell-btn" on:click={() => addHourReminder(item)} title="Напомнить через 1 час">
                      🔔
                    </button>
                    <button class="action-btn edit-btn" on:click={() => startEdit(item)} title="Редактировать">
                      ✏️
                    </button>
                    <button class="action-btn delete-btn" on:click={() => deleteCard(item.id)} title="Удалить">
                      🗑️
                    </button>
                  </div>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/each}
  </div>

  {#if showDeadlineModal}
    <div class="modal-backdrop">
      <div class="modal-window">
        <h4>⏰ Сроки для задачи</h4>
        <p>Вы переносите задачу в секцию <strong>В процессе</strong>. Желаете установить дедлайн?</p>
        
        <input type="datetime-local" bind:value={modalDeadlineValue} class="modal-input" />
        
        <div class="modal-buttons">
          <button class="btn-accent" on:click={saveModalDeadline}>Установить срок</button>
          <button class="btn-secondary" on:click={skipModalDeadline}>Пропустить</button>
        </div>
      </div>
    </div>
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background-color: #0f172a;
    color: #f8fafc;

    color-scheme: dark;
  }

  .app-container {
    max-width: 1300px;
    margin: 0 auto;
    padding: 20px;
    box-sizing: border-box;
  }

  /* Хедер */
  .app-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-bottom: 20px;
    border-bottom: 1px solid #334155;
    margin-bottom: 24px;
  }
  .logo {
    font-size: 22px;
    font-weight: 800;
    letter-spacing: -0.5px;
  }
  .logo span { color: #6366f1; }
  .status-indicator {
    font-size: 12px;
    color: #10b981;
    background: rgba(16, 185, 129, 0.1);
    padding: 4px 10px;
    border-radius: 20px;
  }

  /* Панель добавления */
  .creation-panel {
    background: #1e293b;
    padding: 16px;
    border-radius: 12px;
    border: 1px solid #334155;
    margin-bottom: 20px;
    box-shadow: 0 4px 6px -1px rgba(0,0,0,0.2);
  }
  .input-wrapper {
    display: flex;
    gap: 12px;
    align-items: center;
  }
  .input-wrapper input {
    flex: 1;
    background: #0f172a;
    border: 1px solid #475569;
    padding: 12px 16px;
    border-radius: 8px;
    color: white;
    font-size: 14px;
    outline: none;
    transition: border 0.2s;
  }
  .input-wrapper input:focus { border-color: #6366f1; }
  
  .priority-select {
    padding: 11px;
    background: #0f172a;
    color: white;
    border: 1px solid #475569;
    border-radius: 8px;
    cursor: pointer;
  }

  .add-button {
    background: #4f46e5;
    color: white;
    border: none;
    padding: 12px 24px;
    border-radius: 8px;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.2s;
  }
  .add-button:hover { background: #4338ca; }

  .description-wrapper {
    margin-top: 12px;
  }
  .creation-description {
    width: 100%;
    background: #0f172a;
    border: 1px solid #475569;
    border-radius: 8px;
    padding: 10px 14px;
    color: white;
    font-size: 13px;
    outline: none;
    resize: vertical;
    box-sizing: border-box;
    transition: border 0.2s;
  }
  .creation-description:focus {
    border-color: #6366f1;
  }

  /* Панель сортировки и фильтрации */
  .toolbar {
    display: flex;
    gap: 20px;
    background: #1e293b;
    padding: 12px 20px;
    border-radius: 10px;
    margin-bottom: 24px;
    border: 1px solid #334155;
    align-items: center;
    flex-wrap: wrap;
    box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1);
  }
  .filter-group {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    color: #94a3b8;
  }
  .filter-group select {
    background: #0f172a;
    color: white;
    border: 1px solid #475569;
    padding: 6px 10px;
    border-radius: 6px;
    outline: none;
    cursor: pointer;
  }
  .checkbox-group label {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    color: #f43f5e;
    font-weight: 500;
  }
  .hidden-card {
    display: none !important;
  }

  /* Сетка Kanban */
  .kanban-board {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 20px;
    align-items: start;
  }

  .kanban-column {
    background: #1e293b;
    border-radius: 12px;
    border: 1px solid #334155;
    padding: 16px;
    min-height: 500px;
    display: flex;
    flex-direction: column;
  }

  .column-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    padding-bottom: 8px;
    border-bottom: 2px solid #334155;
  }
  .column-header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 700;
    color: #cbd5e1;
  }
  .badge {
    background: #334155;
    padding: 2px 8px;
    border-radius: 12px;
    font-size: 12px;
    color: #94a3b8;
  }

  .column-body {
    flex: 1;
    min-height: 450px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  /* Карточки задач */
  .card {
    background: #273549;
    border-left: 4px solid #94a3b8;
    border-radius: 8px;
    padding: 14px;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
    transition: transform 0.15s, box-shadow 0.15s;
  }
  .card:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0,0,0,0.25);
  }

  /* Градации приоритетов */
  .card.priority-high { border-left-color: #ef4444; }
  .card.priority-medium { border-left-color: #eab308; }
  .card.priority-low { border-left-color: #10b981; }

  .card-layout {
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }
  .card-main {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
  }
  .card-text {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    line-height: 1.4;
    color: #f1f5f9;
    word-break: break-word;
  }
  .card-description {
    margin: 4px 0 6px 0;
    font-size: 12px;
    line-height: 1.5;
    color: #94a3b8;
    white-space: pre-wrap; /* Чтобы сохранялись переносы строк */
    word-break: break-word;
  }
  .card-deadline {
    font-size: 11px;
    color: #94a3b8;
    background: rgba(255,255,255,0.05);
    padding: 2px 6px;
    border-radius: 4px;
    width: fit-content;
    margin-top: 4px;
  }

  .card-controls {
    display: flex;
    flex-direction: column;
    gap: 4px;
    justify-content: flex-start;
  }
  .action-btn {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    font-size: 14px;
    transition: background 0.2s;
  }
  .action-btn:hover { background: rgba(255,255,255,0.1); }

  /* Режим редактирования внутри карточки */
  .edit-mode {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
  }
  .edit-mode textarea {
    background: #0f172a;
    color: white;
    border: 1px solid #475569;
    border-radius: 6px;
    padding: 8px;
    font-size: 13px;
    outline: none;
  }
  .edit-mode textarea {
    resize: vertical;
  }
  .edit-row {
    display: flex;
    gap: 6px;
  }
  .edit-row input, .edit-row select {
    background: #0f172a;
    color: white;
    border: 1px solid #475569;
    border-radius: 4px;
    padding: 6px;
    font-size: 12px;
    flex: 1;
    outline: none;
    color-scheme: dark;
  }
  .edit-actions {
    display: flex;
    gap: 6px;
  }
  .edit-actions button {
    flex: 1;
    padding: 8px;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 600;
  }
  .btn-save { background: #10b981; color: white; }
  .btn-cancel { background: #475569; color: white; }

  /* Модальное окно */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(15, 23, 42, 0.8);
    backdrop-filter: blur(4px);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 999;
  }
  .modal-window {
    background: #1e293b;
    border: 1px solid #475569;
    border-radius: 12px;
    padding: 24px;
    max-width: 400px;
    width: 100%;
    box-shadow: 0 20px 25px -5px rgba(0,0,0,0.5);
  }
  .modal-window h4 { margin: 0 0 10px 0; font-size: 18px; color: white;}
  .modal-window p { font-size: 14px; color: #94a3b8; margin-bottom: 16px; line-height: 1.4;}
  .modal-input {
    width: 100%;
    background: #0f172a;
    border: 1px solid #475569;
    color: white;
    padding: 10px;
    border-radius: 6px;
    margin-bottom: 20px;
    box-sizing: border-box;
    color-scheme: dark;
  }
  .modal-buttons {
    display: flex;
    gap: 12px;
  }
  .modal-buttons button {
    flex: 1;
    padding: 10px;
    border: none;
    border-radius: 6px;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-accent { background: #4f46e5; color: white; }
  .btn-secondary { background: #334155; color: #cbd5e1; }
</style>