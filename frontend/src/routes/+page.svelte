<script>
  import { onMount } from 'svelte';
  import { dndzone } from 'svelte-dnd-action';
  import { flip } from 'svelte/animate';
  import { slide } from 'svelte/transition';
  import { db } from '$lib/db';
  import { fetchNotesFromBackend, sendNoteToBackend, updateNoteInBackend } from '$lib/api';

  const flipDurationMs = 200;
  let newNote = "";
  let newDescription = "";
  let deadline = "";
  let newPriority = "low";

  // Управление панелями интерфейса
  let showCreationPanel = false;
  let showSettingsDrawer = false; // Переключено на боковую шторку

  // Режим редактирования карточек
  let editingId = null;
  let editContent = "";
  let editDescription = "";
  let editDeadline = "";
  let editPriority = "low";

  // Модалка для дедлайна
  let showDeadlineModal = false;
  let modalNote = null;
  let modalDeadlineValue = "";

  // Сортировка и фильтрация
  let filterPriority = "all";
  let filterUrgent = false;
  let sortBy = "none";

  // Кастомизация пространства
  let currentTheme = "default";
  let currentCardStyle = "style-default";
  let usePhotoBackground = false;
  let bgImageUrl = "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=2560&q=90";

  // Состояния для динамического поиска фонов
  let searchQuery = "";
  let isSearchingImages = false;
  let searchError = "";

  // Стартовый набор качественных обоев по умолчанию
  const defaultWallpapers = [
    { id: 'abstract', name: 'Абстракция', thumb: 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=400&q=80', url: 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=2560&q=90' },
    { id: 'cyber', name: 'Киберпанк', thumb: 'https://images.unsplash.com/photo-1509198397868-475647b2a1e5?auto=format&fit=crop&w=400&q=80', url: 'https://images.unsplash.com/photo-1509198397868-475647b2a1e5?auto=format&fit=crop&w=2560&q=90' },
    { id: 'space', name: 'Космос', thumb: 'https://images.unsplash.com/photo-1506318137071-a8e063b4bec0?auto=format&fit=crop&w=400&q=80', url: 'https://images.unsplash.com/photo-1506318137071-a8e063b4bec0?auto=format&fit=crop&w=2560&q=90' },
    { id: 'nature', name: 'Природа', thumb: 'https://images.unsplash.com/photo-1469474968028-56623f02e42e?auto=format&fit=crop&w=400&q=80', url: 'https://images.unsplash.com/photo-1469474968028-56623f02e42e?auto=format&fit=crop&w=2560&q=90' }
  ];
  let searchedWallpapers = [...defaultWallpapers];

  // Динамическая функция поиска через ключевые слова (Unsplash NAPI)
  async function searchBackgrounds() {
    if (!searchQuery.trim()) return;
    isSearchingImages = true;
    searchError = "";
    
    try {
      const response = await fetch(`/api/backgrounds?query=${encodeURIComponent(searchQuery)}`);
      const data = await response.json();
      
      // Если сервер вернул ошибку (404, 403, 500)
      if (!response.ok) {
        throw new Error(data.error || `Код ответа сервера: ${response.status}`);
      }
      
      if (data.results && data.results.length > 0) {
        searchedWallpapers = data.results.map(img => {
          const baseImgUrl = img.urls.raw.split('?')[0];
          return {
            id: img.id,
            name: img.alt_description || searchQuery,
            thumb: img.urls.small,
            url: `${baseImgUrl}?auto=format&fit=crop&w=2560&q=90`
          };
        });
      } else {
        searchError = "Ничего не найдено. Попробуйте другое слово.";
      }
    } catch (err) {
      console.error("Фронтенд поймал ошибку:", err);
      // Выводим реальный текст ошибки прямо на UI шторки настройки
      searchError = err.message;
    } finally {
      isSearchingImages = false;
    }
  }

  // Реактивный трекинг тем оформления
  $: if (typeof document !== 'undefined') {
    const _triggerTheme = currentTheme;
    const _triggerPhotoMode = usePhotoBackground;
    
    document.body.className = '';
    document.body.classList.add(`theme-${currentTheme}`);
    
    if (usePhotoBackground) {
      document.body.classList.add('has-photo-bg');
    }
    
    localStorage.setItem('sm-theme', currentTheme);
    localStorage.setItem('sm-use-photo-bg', usePhotoBackground ? 'true' : 'false');
  }

  $: if (currentCardStyle && typeof localStorage !== 'undefined') {
    localStorage.setItem('sm-card-style', currentCardStyle);
  }

  function selectWallpaper(url) {
    bgImageUrl = url;
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('sm-bg-image-url', url);
    }
  }

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
      console.warn("Офлайн режим доски:", err);
      await refreshBoardFromIndexedDB();
    }
  }

  async function refreshBoardFromIndexedDB() {
    const allLocal = await db.notes.toArray();
    const weights = { high: 3, medium: 2, low: 1 };
    columns = columns.map(col => {
      let items = allLocal
        .filter(n => n.status === col.id)
        .map(n => ({ ...n, id: String(n.id) }));

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

  function isCardVisible(item, currentPriorityFilter, currentUrgentFilter) {
    if (currentPriorityFilter !== "all" && item.priority !== currentPriorityFilter) return false;
    if (currentUrgentFilter) {
      if (!item.deadline) return false;
      const todayStr = new Date().toISOString().split('T')[0];
      return item.deadline.split('T')[0] <= todayStr;
    }
    return true;
  }

  onMount(async () => {
    currentTheme = localStorage.getItem('sm-theme') || 'default';
    currentCardStyle = localStorage.getItem('sm-card-style') || 'style-default';
    usePhotoBackground = localStorage.getItem('sm-use-photo-bg') === 'true';
    bgImageUrl = localStorage.getItem('sm-bg-image-url') || defaultWallpapers[0].url;
    
    await syncAndLoad();
    setInterval(syncAndLoad, 15000);
  });

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
    await db.notes.put({ ...noteData, id: localId });
    newNote = "";
    newDescription = "";
    newPriority = "low";
    showCreationPanel = false; 
    await refreshBoardFromIndexedDB();
    
    const result = await sendNoteToBackend({ 
      content: noteData.content, 
      description: noteData.description,
      deadline: "", 
      priority: noteData.priority 
    });
    if (result && result.id) {
      await db.notes.delete(localId);
      await db.notes.put({ ...noteData, id: result.id, isSynced: 1 });
      await syncAndLoad();
    }
  }

  async function addHourReminder(item) {
    const oneHourLater = new Date(Date.now() + 60 * 60 * 1000).toISOString();
    await db.notes.update(Number(item.id), { deadline: oneHourLater, notified: 0, isSynced: 0 });
    await refreshBoardFromIndexedDB();

    const updatedNote = await db.notes.get(Number(item.id));
    const success = await updateNoteInBackend(updatedNote);
    if (success) {
      await db.notes.update(Number(item.id), { isSynced: 1 });
    }
  }

  function handleDndConsider(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];
  }

  async function handleDndFinalize(columnId, e) {
    const colIdx = columns.findIndex(c => c.id === columnId);
    columns[colIdx].items = e.detail.items;
    columns = [...columns];

    const triggeredItem = e.detail.info?.id ? e.detail.items.find(i => i.id === e.detail.info.id) : null;
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
      if (columnId === 'done') updatedDeadline = null;

      await db.notes.update(numericId, { status: columnId, deadline: updatedDeadline, isSynced: 0 });
      await refreshBoardFromIndexedDB();

      const fullNote = await db.notes.get(numericId);
      const success = await updateNoteInBackend(fullNote);
      if (success) await db.notes.update(numericId, { isSynced: 1 });
    }
  }

  async function saveModalDeadline() {
    if (!modalNote) return;
    const numericId = Number(modalNote.id);
    const formattedDeadline = modalDeadlineValue ? new Date(modalDeadlineValue).toISOString() : null;
    await db.notes.update(numericId, { status: 'in_progress', deadline: formattedDeadline, notified: 0, isSynced: 0 });
    closeModal();
    await refreshBoardFromIndexedDB();

    const fullNote = await db.notes.get(numericId);
    const success = await updateNoteInBackend(fullNote);
    if (success) await db.notes.update(numericId, { isSynced: 1 });
  }

  async function skipModalDeadline() {
    if (!modalNote) return;
    const numericId = Number(modalNote.id);
    await db.notes.update(numericId, { status: 'in_progress', deadline: null, isSynced: 0 });
    closeModal();
    await refreshBoardFromIndexedDB();

    const fullNote = await db.notes.get(numericId);
    const success = await updateNoteInBackend(fullNote);
    if (success) await db.notes.update(numericId, { isSynced: 1 });
  }

  function closeModal() {
    showDeadlineModal = false;
    modalNote = null;
    modalDeadlineValue = "";
  }

  async function deleteCard(id) {
    const numericId = Number(id);
    await db.notes.delete(numericId);
    await refreshBoardFromIndexedDB();
    try {
      await fetch(`/api/notes?id=${numericId}`, { method: 'DELETE' });
    } catch (e) {
      console.error("Удалено только локально", e);
    }
  }

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
    if (success) await db.notes.update(numericId, { isSynced: 1 });
  }

  function formatDisplayDate(isoString) {
    if (!isoString) return '';
    const d = new Date(isoString);
    return d.toLocaleString('ru-RU', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }
</script>

<div class="app-layout" class:drawer-open={showSettingsDrawer}>
  
  {#if usePhotoBackground && bgImageUrl}
    <div class="fullscreen-bg" style="background-image: url('{bgImageUrl}');"></div>
  {/if}

  <main class="main-content">
    <header class="app-header">
      <div class="logo">🗂 Self<span>Manager</span></div>
      
      <div class="header-actions">
        <div class="status-indicator">Active</div>
        
        <button 
          class="action-toggle-btn create-btn" 
          class:active={showCreationPanel} 
          on:click={() => { showCreationPanel = !showCreationPanel; }}
          title="Новая задача"
        >
          {showCreationPanel ? '✕' : '＋'}
        </button>

        <button 
          class="action-toggle-btn settings-btn" 
          class:active={showSettingsDrawer} 
          on:click={() => { showSettingsDrawer = !showSettingsDrawer; }}
          title="Настройки пространства"
        >
          🎛️
        </button>
      </div>
    </header>

    {#if showCreationPanel}
      <section class="creation-panel" transition:slide={{ duration: 200 }}>
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
    {/if}

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
                class="card priority-{item.priority} {currentCardStyle} {isCardVisible(item, filterPriority, filterUrgent) ? '' : 'hidden-card'}"
              >
                {#if editingId === item.id}
                  <div class="edit-mode">
                    <input type="text" bind:value={editContent} class="edit-title-input" />
                    <textarea bind:value={editDescription} rows="3"></textarea>
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
                      <button class="action-btn bell-btn" on:click={() => addHourReminder(item)}>🔔</button>
                      <button class="action-btn edit-btn" on:click={() => startEdit(item)}>✏️</button>
                      <button class="action-btn delete-btn" on:click={() => deleteCard(item.id)}>🗑️</button>
                    </div>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  </main>

  <aside class="settings-drawer">
    <div class="drawer-header">
      <h3>Настройки пространства</h3>
      <button class="close-drawer-btn" on:click={() => showSettingsDrawer = false}>✕</button>
    </div>

    <div class="drawer-content">
      <div class="dropdown-section">
        <span class="section-title">🔍 Сортировка и Фильтры</span>
        <div class="controls-row">
          <select bind:value={filterPriority}>
            <option value="all">Все приоритеты</option>
            <option value="low">🟢 Низкий</option>
            <option value="medium">🟡 Средний</option>
            <option value="high">🔴 Высокий</option>
          </select>
          <select bind:value={sortBy} on:change={refreshBoardFromIndexedDB}>
            <option value="none">По порядку</option>
            <option value="priority">По приоритету</option>
            <option value="deadline">По дедлайну</option>
          </select>
        </div>
        <div class="filter-group checkbox-group">
          <label><input type="checkbox" bind:checked={filterUrgent}> 🔥 Только горящие</label>
        </div>
      </div>

      <hr class="dropdown-divider" />

      <div class="dropdown-section">
        <span class="section-title">🎨 Цветовая тема</span>
        <div class="pinterest-grid">
          <button class="preview-tile" class:selected={currentTheme === 'default'} on:click={() => currentTheme = 'default'}>
            <div class="mock-window tile-theme-default"><div class="mock-card"></div></div>
            <span>Слейт</span>
          </button>
          <button class="preview-tile" class:selected={currentTheme === 'fantasy'} on:click={() => currentTheme = 'fantasy'}>
            <div class="mock-window tile-theme-fantasy"><div class="mock-card"></div></div>
            <span>Фэнтези</span>
          </button>
          <button class="preview-tile" class:selected={currentTheme === 'cyberpunk'} on:click={() => currentTheme = 'cyberpunk'}>
            <div class="mock-window tile-theme-cyberpunk"><div class="mock-card"></div></div>
            <span>Киберпанк</span>
          </button>
          <button class="preview-tile" class:selected={currentTheme === 'matrix'} on:click={() => currentTheme = 'matrix'}>
            <div class="mock-window tile-theme-matrix"><div class="mock-card"></div></div>
            <span>Матрица</span>
          </button>
        </div>
      </div>

      <hr class="dropdown-divider" />

      <div class="dropdown-section">
        <span class="section-title">🃏 Дизайн карточек</span>
        <div class="pinterest-grid">
          <button class="preview-tile" class:selected={currentCardStyle === 'style-default'} on:click={() => currentCardStyle = 'style-default'}>
            <div class="mock-card-preview val-default">Текст</div>
            <span>Стандарт</span>
          </button>
          <button class="preview-tile" class:selected={currentCardStyle === 'style-neon'} on:click={() => currentCardStyle = 'style-neon'}>
            <div class="mock-card-preview val-neon">Неон</div>
            <span>Неон</span>
          </button>
          <button class="preview-tile" class:selected={currentCardStyle === 'style-glass'} on:click={() => currentCardStyle = 'style-glass'}>
            <div class="mock-card-preview val-glass">Стекло</div>
            <span>Стекло</span>
          </button>
          <button class="preview-tile" class:selected={currentCardStyle === 'style-minimal'} on:click={() => currentCardStyle = 'style-minimal'}>
            <div class="mock-card-preview val-minimal">Мини</div>
            <span>Мини</span>
          </button>
        </div>
      </div>

      <hr class="dropdown-divider" />

      <div class="dropdown-section">
        <span class="section-title">🖼️ Интерактивные HD-Обои</span>
        <div class="bg-toggle-container">
          <label class="switch-label">
            <input type="checkbox" bind:checked={usePhotoBackground}>
            Включить фото-фоны
          </label>
        </div>
        
        {#if usePhotoBackground}
          <div class="search-box-container" transition:slide={{ duration: 150 }}>
            <div class="search-input-group">
              <input 
                type="text" 
                bind:value={searchQuery} 
                placeholder="Что искать? (напр. Футбол, Горы...)" 
                on:keydown={(e) => e.key === 'Enter' && searchBackgrounds()}
              />
              <button on:click={searchBackgrounds} disabled={isSearchingImages}>
                {isSearchingImages ? '...' : '🔍'}
              </button>
            </div>

            {#if searchError}
              <span class="search-error-msg">{searchError}</span>
            {/if}

            <div class="wallpaper-gallery">
              {#each searchedWallpapers as wp (wp.id)}
                <button 
                  class="wallpaper-item" 
                  class:active={bgImageUrl === wp.url}
                  style="background-image: url('{wp.thumb}');" 
                  on:click={() => selectWallpaper(wp.url)}
                  title={wp.name}
                >
                  <span class="wp-name">{wp.name}</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    </div>
  </aside>

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
</div>

<style>
  :global(body) {
    margin: 0;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
    background-color: #0f172a;
    color: #f8fafc;
    overflow-x: hidden;
    -webkit-font-smoothing: antialiased;
  }

  /* Макет с поддержкой сдвига контента (Вариант А) */
  .app-layout {
    display: flex;
    width: 100vw;
    min-height: 100vh;
    position: relative;
    overflow-x: hidden;
  }

  .main-content {
    flex: 1;
    width: 100%;
    max-width: 1400px;
    margin: 0 auto;
    padding: 20px;
    box-sizing: border-box;
    transition: padding-right 0.32s cubic-bezier(0.4, 0, 0.2, 1), max-width 0.32s ease;
  }

  /* Сжатие рабочей области доски при открытии шторки */
  .app-layout.drawer-open .main-content {
    padding-right: 450px; 
    max-width: calc(100vw - 20px);
  }

  /* Полноэкранный бэкграунд высокой четкости */
  .fullscreen-bg {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background-size: cover;
    background-position: center;
    background-repeat: no-repeat;
    z-index: -2;
    filter: brightness(0.35); /* Мягкое затемнение для читаемости текста карточек */
    transition: background-image 0.4s ease-in-out;
    pointer-events: none;
  }

  /* Матовое стекло на элементах при активных фотообоях */
  :global(body.has-photo-bg) .kanban-column,
  :global(body.has-photo-bg) .creation-panel,
  :global(body.has-photo-bg) .settings-drawer,
  :global(body.has-photo-bg) .modal-window {
    backdrop-filter: blur(18px) !important;
    -webkit-backdrop-filter: blur(18px) !important;
  }
  :global(body.has-photo-bg.theme-default) .kanban-column,
  :global(body.has-photo-bg.theme-default) .creation-panel,
  :global(body.has-photo-bg.theme-default) .settings-drawer { background: rgba(30, 41, 59, 0.73) !important; }
  :global(body.has-photo-bg.theme-fantasy) .kanban-column,
  :global(body.has-photo-bg.theme-fantasy) .settings-drawer { background: rgba(27, 21, 17, 0.78) !important; }
  :global(body.has-photo-bg.theme-cyberpunk) .kanban-column,
  :global(body.has-photo-bg.theme-cyberpunk) .settings-drawer { background: rgba(21, 2, 36, 0.7) !important; }
  :global(body.has-photo-bg.theme-matrix) .kanban-column,
  :global(body.has-photo-bg.theme-matrix) .settings-drawer { background: rgba(0, 0, 0, 0.85) !important; }

  /* РЕАЛИЗАЦИЯ ВЫДВИЖНОЙ БОКОВОЙ ПАНЕЛИ (DRAWER) */
  .settings-drawer {
    position: fixed;
    top: 0;
    right: -420px;
    width: 420px;
    height: 100vh;
    background: #1e293b;
    border-left: 1px solid #334155;
    box-shadow: -8px 0 25px rgba(0, 0, 0, 0.4);
    transition: right 0.32s cubic-bezier(0.4, 0, 0.2, 1);
    z-index: 1000;
    display: flex;
    flex-direction: column;
  }

  .app-layout.drawer-open .settings-drawer {
    right: 0;
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 18px 20px;
    border-bottom: 1px solid #334155;
  }
  .drawer-header h3 { margin: 0; font-size: 16px; font-weight: 700; color: #f1f5f9; }
  .close-drawer-btn { background: transparent; border: none; color: #94a3b8; font-size: 18px; cursor: pointer; }
  .close-drawer-btn:hover { color: white; }

  .drawer-content {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  /* СТИЛИ ПАНЕЛИ ПОИСКА ФОНОВ */
  .search-box-container { display: flex; flex-direction: column; gap: 8px; margin-top: 10px; }
  .search-input-group { display: flex; gap: 6px; }
  .search-input-group input { flex: 1; background: #0f172a; border: 1px solid #475569; border-radius: 6px; padding: 8px 12px; color: white; font-size: 13px; outline: none; }
  .search-input-group button { background: #4f46e5; border: none; color: white; padding: 8px 12px; border-radius: 6px; cursor: pointer; font-weight: 600; }
  .search-input-group button:hover { background: #6366f1; }
  .search-error-msg { font-size: 11px; color: #ef4444; }

  .wallpaper-gallery {
    display: grid; 
    grid-template-columns: repeat(2, 1fr); /* 2 колонки вместо 3 — картинки крупнее */
    gap: 10px; 
    margin-top: 8px;
    max-height: none; /* Убираем жесткий лимит высоты */
    overflow-y: visible; /* Уничтожаем внутренний скроллбар */
    padding-right: 0;
  }
  .wallpaper-item {
    width: 100%;
    aspect-ratio: 16 / 10; /* Устанавливаем горизонтальные пропорции */
    height: auto; /* Сбрасываем фиксированные 60px */
    border-radius: 8px; 
    background-size: cover; 
    background-position: center;
    border: 2px solid #334155; 
    cursor: pointer; 
    position: relative; 
    display: flex;
    align-items: flex-end; 
    padding: 0; 
    box-sizing: border-box; 
    transition: transform 0.2s ease, border-color 0.2s ease;
    overflow: hidden;
  }
  .wallpaper-item:hover { border-color: #6366f1; transform: scale(1.02); }
  .wallpaper-item.active { border-color: #4f46e5; box-shadow: 0 0 8px rgba(79, 70, 229, 0.6); }
  .wp-name {
    font-size: 10px; /* Сделали чуть более читаемым */
    font-weight: 700; 
    color: white; 
    background: rgba(15, 23, 42, 0.85);
    width: 100%; 
    text-align: center; 
    padding: 4px 0;
    white-space: nowrap; 
    overflow: hidden; 
    text-overflow: ellipsis;
  }

  /* СИСТЕМА ДИНАМИЧЕСКИХ ТЕМ */
  /* --- ФЭНТЕЗИ --- */
  :global(body.theme-fantasy) { background-color: #120e0b !important; color: #e5d5c5 !important; }
  :global(body.theme-fantasy) .app-header { border-bottom-color: #423429; }
  :global(body.theme-fantasy) .logo span { color: #876843; }
  :global(body.theme-fantasy) .kanban-column,
  :global(body.theme-fantasy) .creation-panel,
  :global(body.theme-fantasy) .modal-window,
  :global(body.theme-fantasy) .settings-drawer { background: #1b1511; border-color: #423429; }
  :global(body.theme-fantasy) .column-header { border-bottom-color: #423429; }
  :global(body.theme-fantasy) .badge { background: #423429; color: #e5d5c5; }
  :global(body.theme-fantasy) .card.style-default { background: #28201a; border-color: #423429; }
  :global(body.theme-fantasy) .card-text { color: #f2e6da; }
  :global(body.theme-fantasy) input, :global(body.theme-fantasy) select, :global(body.theme-fantasy) textarea { background: #120e0b !important; border-color: #423429 !important; color: #e5d5c5 !important; }
  :global(body.theme-fantasy) .action-toggle-btn { background: #28201a; border-color: #423429; color: #e5d5c5; }
  :global(body.theme-fantasy) .add-button, :global(body.theme-fantasy) .btn-accent { background: #876843 !important; color: #120e0b !important; font-weight: 700; }

  /* --- КИБЕРПАНК --- */
  :global(body.theme-cyberpunk) { background-color: #0c0214 !important; color: #00ffcc !important; }
  :global(body.theme-cyberpunk) .app-header { border-bottom-color: #3d055a; }
  :global(body.theme-cyberpunk) .logo span { color: #ff007f; text-shadow: 0 0 8px #ff007f; }
  :global(body.theme-cyberpunk) .kanban-column, :global(body.theme-cyberpunk) .settings-drawer { background: #150224; border-color: #3d055a !important; }
  :global(body.theme-cyberpunk) .column-header { border-bottom-color: #ff007f; }
  :global(body.theme-cyberpunk) .badge { background: #3d055a; color: #00ffcc; border: 1px solid #ff007f; }
  :global(body.theme-cyberpunk) .card.style-default { background: #210535; border: 1px solid #3d055a; }
  :global(body.theme-cyberpunk) input, :global(body.theme-cyberpunk) select, :global(body.theme-cyberpunk) textarea { background: #0c0214 !important; border-color: #3d055a !important; color: #00ffcc !important; }
  :global(body.theme-cyberpunk) .add-button, :global(body.theme-cyberpunk) .btn-accent { background: #ff007f !important; color: white !important; box-shadow: 0 0 10px #ff007f; }

  /* --- МАТРИЦА --- */
  :global(body.theme-matrix) { background-color: #000000 !important; color: #00ff00 !important; font-family: 'Courier New', monospace !important; }
  :global(body.theme-matrix) * { font-family: 'Courier New', monospace !important; }
  :global(body.theme-matrix) .kanban-column, :global(body.theme-matrix) .settings-drawer { background: #000; border: 1px solid #00ff00 !important; }
  :global(body.theme-matrix) .column-header { border-bottom-color: #00ff00; }
  :global(body.theme-matrix) .card.style-default { background: #000; border: 1px solid #004400; }
  :global(body.theme-matrix) input, :global(body.theme-matrix) select, :global(body.theme-matrix) textarea { background: #000 !important; border-color: #00ff00 !important; color: #00ff00 !important; }
  :global(body.theme-matrix) .add-button, :global(body.theme-matrix) .btn-accent { background: #00ff00 !important; color: black !important; font-weight: bold; }

  /* БАЗОВАЯ РАЗМЕТКА ИНТЕРФЕЙСА ДОСКИ */
  .app-header { display: flex; justify-content: space-between; align-items: center; padding-bottom: 16px; border-bottom: 1px solid #334155; margin-bottom: 24px; }
  .logo { font-size: 22px; font-weight: 800; }
  .logo span { color: #6366f1; }
  .header-actions { display: flex; align-items: center; gap: 12px; }
  .status-indicator { font-size: 11px; color: #10b981; background: rgba(16, 185, 129, 0.1); padding: 6px 12px; border-radius: 20px; font-weight: 600; }

  .action-toggle-btn { width: 42px; height: 42px; display: flex; align-items: center; justify-content: center; background: #1e293b; border: 1px solid #334155; border-radius: 50%; color: white; cursor: pointer; transition: all 0.2s; }
  .action-toggle-btn:hover, .action-toggle-btn.active { background: #4f46e5; border-color: #6366f1; transform: scale(1.05); }

  .dropdown-section { display: flex; flex-direction: column; gap: 8px; }
  .section-title { font-size: 11px; text-transform: uppercase; letter-spacing: 0.6px; color: #64748b; font-weight: 700; }
  .controls-row { display: flex; gap: 8px; }
  .controls-row select { flex: 1; background: #0f172a; color: white; border: 1px solid #475569; padding: 8px; border-radius: 6px; font-size: 12px; cursor: pointer; }
  .checkbox-group label { display: flex; align-items: center; gap: 8px; cursor: pointer; color: #f43f5e; font-weight: 600; font-size: 12px; }
  .dropdown-divider { border: 0; border-top: 1px solid #334155; margin: 0; }

  .pinterest-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 8px; }
  .preview-tile { background: #0f172a; border: 2px solid #334155; border-radius: 8px; padding: 6px; cursor: pointer; display: flex; flex-direction: column; align-items: center; gap: 4px; }
  .preview-tile span { font-size: 11px; font-weight: 600; color: #cbd5e1; }
  .preview-tile.selected { border-color: #6366f1; }

  .mock-window { width: 100%; height: 34px; border-radius: 6px; position: relative; overflow: hidden; }
  .mock-card { width: 45%; height: 16px; border-radius: 3px; position: absolute; top: 8px; left: 8px; }
  .tile-theme-default { background: #0f172a; }
  .tile-theme-default .mock-card { background: #273549; border-left: 2px solid #10b981; }
  .tile-theme-fantasy { background: #120e0b; }
  .tile-theme-fantasy .mock-card { background: #28201a; border-left: 2px solid #876843; }
  .tile-theme-cyberpunk { background: #0c0214; border: 1px solid #ff007f; }
  .tile-theme-cyberpunk .mock-card { background: #24053d; border-left: 2px solid #00ffcc; }
  .tile-theme-matrix { background: #000; border: 1px solid #00ff00; }
  .tile-theme-matrix .mock-card { background: #000; border: 1px solid #005500; }

  .mock-card-preview { width: 100%; height: 34px; border-radius: 6px; display: flex; align-items: center; justify-content: center; font-size: 10px; font-weight: 700; }
  .val-default { background: #273549; border-left: 3px solid #6366f1; color: white; }
  .val-neon { background: rgba(239, 68, 68, 0.04); border: 1px solid #ef4444; color: #ef4444; }
  .val-glass { background: rgba(255, 255, 255, 0.05); border: 1px solid rgba(255, 255, 255, 0.1); color: #cbd5e1; }
  .val-minimal { background: transparent; border: 1px solid #475569; color: #94a3b8; }

  .switch-label { display: flex; align-items: center; gap: 8px; font-size: 12px; font-weight: 600; cursor: pointer; color: #cbd5e1; }

  /* Форма добавления тасок */
  .creation-panel { background: #1e293b; padding: 16px; border-radius: 10px; border: 1px solid #334155; margin-bottom: 24px; }
  .input-wrapper { display: flex; gap: 10px; }
  .input-wrapper input { flex: 1; background: #0f172a; border: 1px solid #475569; padding: 10px; border-radius: 6px; color: white; outline: none; }
  .priority-select { padding: 8px; background: #0f172a; color: white; border: 1px solid #475569; border-radius: 6px; }
  .add-button { background: #4f46e5; color: white; border: none; padding: 10px 20px; border-radius: 6px; font-weight: 600; cursor: pointer; }
  .description-wrapper { margin-top: 10px; }
  .creation-description { width: 100%; background: #0f172a; border: 1px solid #475569; border-radius: 6px; padding: 10px; color: white; font-size: 13px; resize: vertical; box-sizing: border-box; }

  /* Канбан сетка */
  .kanban-board { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; align-items: start; }
  .kanban-column { background: #1e293b; border-radius: 10px; border: 1px solid #334155; padding: 14px; min-height: 500px; display: flex; flex-direction: column; }
  .column-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; padding-bottom: 6px; border-bottom: 2px solid #334155; }
  .column-header h3 { margin: 0; font-size: 15px; font-weight: 700; color: #cbd5e1; }
  .badge { background: #334155; padding: 2px 8px; border-radius: 10px; font-size: 11px; color: #94a3b8; }
  .column-body { flex: 1; min-height: 450px; display: flex; flex-direction: column; gap: 8px; }

  /* Карточки */
  .card { background: #273549; border-left: 4px solid #94a3b8; border-radius: 6px; padding: 12px; box-shadow: 0 2px 4px rgba(0,0,0,0.15); }
  .card.priority-high { border-left-color: #ef4444; }
  .card.priority-medium { border-left-color: #eab308; }
  .card.priority-low { border-left-color: #10b981; }
  .card-layout { display: flex; justify-content: space-between; gap: 10px; }
  .card-main { display: flex; flex-direction: column; gap: 4px; flex: 1; }
  .card-text { margin: 0; font-size: 13px; font-weight: 600; color: #f1f5f9; word-break: break-word; }
  .card-description { margin: 2px 0; font-size: 11px; color: #94a3b8; white-space: pre-wrap; word-break: break-word; }
  .card-deadline { font-size: 10px; color: #94a3b8; background: rgba(255,255,255,0.05); padding: 2px 4px; border-radius: 3px; width: fit-content; }
  .card-controls { display: flex; flex-direction: column; gap: 2px; }
  .action-btn { background: transparent; border: none; cursor: pointer; padding: 4px; font-size: 12px; }
  .hidden-card { display: none !important; }

  /* Стили карточек продвинутые */
  .card.style-neon { background: #0f172a; border: 1px solid #334155; }
  .card.style-neon.priority-high { box-shadow: 0 0 10px rgba(239, 68, 68, 0.3); border-color: #ef4444; }
  .card.style-neon.priority-medium { box-shadow: 0 0 10px rgba(234, 179, 8, 0.3); border-color: #eab308; }
  .card.style-neon.priority-low { box-shadow: 0 0 10px rgba(16, 185, 129, 0.3); border-color: #10b981; }

  .card.style-glass { background: rgba(255, 255, 255, 0.03); border: 1px solid rgba(255, 255, 255, 0.08); }
  .card.style-minimal { background: transparent; border: 1px solid #334155; border-left: none; }
  .card.style-minimal.priority-high { border-top: 3px solid #ef4444; }
  .card.style-minimal.priority-medium { border-top: 3px solid #eab308; }
  .card.style-minimal.priority-low { border-top: 3px solid #10b981; }

  /* Модальное окно */
  .modal-backdrop { position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(15, 23, 42, 0.75); display: flex; justify-content: center; align-items: center; z-index: 999; }
  .modal-window { background: #1e293b; border: 1px solid #475569; border-radius: 10px; padding: 20px; max-width: 380px; width: 100%; }
  .modal-window h4 { margin: 0 0 8px 0; color: white; }
  .modal-window p { font-size: 13px; color: #94a3b8; }
  .modal-input { width: 100%; background: #0f172a; border: 1px solid #475569; color: white; padding: 8px; border-radius: 6px; margin-bottom: 16px; box-sizing: border-box; color-scheme: dark; }
  .modal-buttons { display: flex; gap: 10px; }
  .modal-buttons button { flex: 1; padding: 8px; border: none; border-radius: 6px; font-weight: 600; cursor: pointer; }
  .btn-accent { background: #4f46e5; color: white; }
  .btn-secondary { background: #334155; color: #cbd5e1; }

  /* Адаптив под мобилки */
  @media (max-width: 768px) {
    .app-layout.drawer-open .main-content { padding-right: 20px; }
    .settings-drawer { width: 100%; right: -100%; }
    .kanban-board { grid-template-columns: 1fr; }
  }
</style>