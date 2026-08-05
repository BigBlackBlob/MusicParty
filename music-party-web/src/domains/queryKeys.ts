export const queryKeys = {
  rooms: {
    all: ['rooms'] as const
  },
  platforms: {
    byRoom: (roomId: string) => ['platforms', roomId] as const
  },
  search: {
    tracks: (roomId: string, platform: string, keyword: string, page: number) =>
      ['search', 'tracks', roomId, platform, keyword, page] as const,
    albums: (roomId: string, platform: string, keyword: string) =>
      ['search', 'albums', roomId, platform, keyword] as const,
    albumTracks: (roomId: string, platform: string, albumId: string) =>
      ['search', 'album-tracks', roomId, platform, albumId] as const,
    externalPlaylist: (platform: string, playlistId: string, page: number) =>
      ['search', 'external-playlist', platform, playlistId, page] as const
  },
  playlists: {
    personal: () => ['playlists', 'personal'] as const,
    room: (roomId: string) => ['playlists', 'room', roomId] as const,
    personalTracks: (playlistId: string) => ['playlists', 'personal', playlistId, 'tracks'] as const,
    roomTracks: (roomId: string, playlistId: string) => ['playlists', 'room', roomId, playlistId, 'tracks'] as const
  },
  members: {
    room: (roomId: string) => ['members', roomId] as const
  },
  invites: {
    room: (roomId: string) => ['invites', roomId] as const
  },
  admin: {
    subsonic: (roomId: string) => ['admin', 'subsonic', roomId] as const,
    localTracks: () => ['admin', 'local-tracks'] as const,
    localUploadAccess: () => ['admin', 'local-upload-access'] as const
  }
} as const
