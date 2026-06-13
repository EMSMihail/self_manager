import Dexie from 'dexie';

export const db = new Dexie('SelfManagerDB');
db.version(1).stores({
  notes: '++id, content, deadline, notified, created_at, isSynced' // isSynced поможет нам понять, что улетело на сервер
});