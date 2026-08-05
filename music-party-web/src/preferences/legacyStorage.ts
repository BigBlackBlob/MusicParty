export const obsoleteStorageKeys = [
  'mp_session_token',
  'mp_account_username',
  'mp_connection_profile',
  'mp_room_access_tokens',
  'mp_search_songs',
  'mp_search_albums',
  'mp_search_playlist_songs',
  'mp_search_playlist_id',
  'mp_search_playlist_page'
] as const

export function removeObsoleteStorage(storage: Pick<Storage, 'removeItem'> = localStorage): void {
  for (const key of obsoleteStorageKeys) storage.removeItem(key)
}
