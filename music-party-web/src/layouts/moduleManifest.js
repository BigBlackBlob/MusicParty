import { defineAsyncComponent } from 'vue'

export const MODULE_MANIFEST = {
  userlist: {
    id: 'userlist',
    label: { en: 'Users', zh: '用户列表' },
    icon: 'group',
    component: defineAsyncComponent(() => import('@/components/modules/UserListModule.vue')),
    minWidthPx: 280,
    maxInstances: 1,
    removable: true,
  },
  nowplaying: {
    id: 'nowplaying',
    label: { en: 'Now Playing', zh: '正在播放' },
    icon: 'play_circle',
    component: defineAsyncComponent(() => import('@/components/modules/NowPlayingModule.vue')),
    minWidthPx: 300,
    maxInstances: 1,
    removable: false,   // 播放器始终存在
  },
  lyrics: {
    id: 'lyrics',
    label: { en: 'Lyrics', zh: '歌词' },
    icon: 'lyrics',
    component: defineAsyncComponent(() => import('@/components/modules/LyricsModule.vue')),
    minWidthPx: 320,
    maxInstances: 1,
    removable: true,
  },
  queue: {
    id: 'queue',
    label: { en: 'Queue', zh: '播放队列' },
    icon: 'queue_music',
    component: defineAsyncComponent(() => import('@/components/modules/QueueModule.vue')),
    minWidthPx: 280,
    maxInstances: 1,
    removable: true,
  },
  chat: {
    id: 'chat',
    label: { en: 'Chat', zh: '聊天' },
    icon: 'chat',
    component: defineAsyncComponent(() => import('@/components/modules/ChatModule.vue')),
    minWidthPx: 280,
    maxInstances: 1,
    removable: true,
  },
  playlists: {
    id: 'playlists',
    label: { en: 'Playlists', zh: '歌单' },
    icon: 'library_music',
    component: defineAsyncComponent(() => import('@/components/modules/RoomPlaylistsModule.vue')),
    minWidthPx: 320,
    maxInstances: 1,
    removable: true,
  },
}
