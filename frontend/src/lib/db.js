import Dexie from 'dexie';

export const db = new Dexie('SelfManagerDB');
db.version(2).stores({
  notes: '++id, content, deadline, notified, created_at, isSynced, status' 
});