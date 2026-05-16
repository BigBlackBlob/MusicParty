export const makeAlbumSongSelectionKey = (platform, albumId, songId) => `${platform}:${albumId}:${songId}`;

export const isAlbumSongSelected = (selected, platform, albumId, songId) =>
  selected.has(makeAlbumSongSelectionKey(platform, albumId, songId));

export const toggleAlbumSongSelection = (selected, platform, albumId, songId) => {
  const key = makeAlbumSongSelectionKey(platform, albumId, songId);
  if (selected.has(key)) {
    selected.delete(key);
  } else {
    selected.add(key);
  }
};

export const addAlbumSelections = (selected, platform, albumId, songs) => {
  songs.forEach(song => selected.add(makeAlbumSongSelectionKey(platform, albumId, song.id)));
};

export const clearAlbumSelections = (selected, platform, albumId, songs) => {
  songs.forEach(song => selected.delete(makeAlbumSongSelectionKey(platform, albumId, song.id)));
};

export const selectedAlbumSongs = (selected, platform, albumId, songs) =>
  songs.filter(song => isAlbumSongSelected(selected, platform, albumId, song.id));

export const hasAlbumSelections = (selected, platform, albumId, songs) =>
  songs.some(song => isAlbumSongSelected(selected, platform, albumId, song.id));
