export default {
  app: {
    lounge: 'Lounge',
    brand: 'Music Party'
  },
  common: {
    close: 'Close',
    export: 'Export',
    unknownArtist: 'Unknown Artist',
    by: 'by',
    cancel: 'Cancel',
    confirm: 'Confirm',
    back: '< Back',
    or: 'OR',
    error: 'Error'
  },
  auth: {
    initializeTitle: 'Initialize System',
    initializeDesc: 'Please configure room access.',
    accessTitle: 'Security Access',
    accessDesc: 'Restricted area. Enter passcode.',
    setPasswordPlaceholder: 'Set new password',
    inputPasswordPlaceholder: 'Input password',
    verifying: 'Verifying...',
    confirmPassword: 'Confirm Password',
    unlock: 'Unlock',
    setPasswordProtection: 'Set Password Protection',
    noPasswordPublic: 'No Password (Public)',
    errors: {
      connectionFailed: 'Connection failed',
      invalidPassword: 'Invalid password',
      passwordEmpty: 'Password cannot be empty',
      setupFailed: 'Setup failed'
    }
  },
  settings: {
    title: 'Settings',
    theme: 'Theme',
    dark: 'Dark Mode',
    light: 'Light Mode',
    language: 'Language',
    onlineMembers: 'Online Members',
    active: 'Active',
    guest: 'Guest',
    member: 'Member',
    mainSize: 'Main Size',
    compact: 'Compact',
    standard: 'Standard',
    large: 'Large',
    globalZoom: 'Global Zoom',
    connection: 'Connection',
    connected: 'Connected',
    disconnected: 'Disconnected',
    autoLiteMode: 'Auto Lite Mode',
    liteMode: 'Lite Mode',
    on: 'On',
    off: 'Off',
    english: 'English',
    chinese: 'Chinese',
    toggleTheme: 'Toggle theme',
    moreActiveUsers: '{count} more active users'
  },
  nav: {
    nowPlaying: 'Now',
    queue: 'Queue',
    search: 'Search',
    chat: 'Chat'
  },
  player: {
    play: 'Play',
    pause: 'Pause',
    next: 'Next',
    prev: 'Prev',
    shuffle: 'Shuffle',
    like: 'Like',
    unlike: 'Remove from liked',
    volume: 'Volume',
    muted: 'Muted',
    buffering: 'Buffering',
    loading: 'Loading',
    playbackError: 'Playback Error',
    loadError: 'Could not load the current track.',
    closeLyrics: 'Close Lyrics',
    onlyRequester: 'Only the requester can seek',
    seek: 'Seek',
    prevUnavailable: 'Previous unavailable',
    requestedBy: 'by {name}',
    waiting: 'Waiting for music',
    waitingFirstTrack: 'Waiting for the first track',
    nowPlayingIn: 'Now playing in',
    requestedByShort: 'Req',
    listenersShort: 'Listeners'
  },
  queue: {
    title: 'Play Queue',
    liked: 'Liked Songs',
    upNext: 'Up Next',
    tracks: 'Tracks',
    selected: 'selected',
    all: 'All',
    top: 'Top',
    remove: 'Remove',
    noLiked: 'No liked tracks',
    likedDesc: 'Favorites will stay here.',
    empty: 'Queue is empty',
    emptyDesc: 'Search to add the next track.',
    clearQueue: 'Clear Queue',
    clearConfirm: 'Remove all tracks from the queue?',
    deleteConfirm: 'Remove {count} tracks?',
    removeAll: 'Remove All',
    cancel: 'Cancel',
    loading: 'Loading',
    failed: 'Failed',
    playNext: 'Play next',
    selectTracks: 'Select tracks',
    exportLiked: 'Export liked songs'
  },
  search: {
    placeholder: 'Search for a track...',
    adminPlaceholder: 'Enter admin password',
    title: 'Search',
    discoverMusic: 'Discover Music',
    platforms: 'Platforms',
    noResults: 'No results found',
    noResultsDesc: 'Try a different keyword or platform.',
    searchAndAdd: 'Search and add',
    song: 'Song',
    album: 'Album',
    loading: 'Searching...',
    noAlbums: 'No albums found',
    searchAlbums: 'Search Albums',
    tryDifferent: 'Try a different keyword.',
    albumHint: 'Netease supports adding by album.',
    searchSongs: 'Search Songs',
    startHint: 'Select a platform and enter a keyword.',
    addAlbum: 'Add Album',
    addSong: 'Add Song',
    searchAria: 'Search',
    platformAria: 'Platforms',
    failed: 'Search failed',
    adminExecuted: 'Admin command executed',
    adminFailed: 'Access denied or command failed'
  },
  playlist: {
    fetchPlaylistsFailed: 'Fetch playlists failed',
    userSearchFailed: 'User search failed',
    fetchSongsFailed: 'Fetch songs failed'
  },
  userList: {
    me: 'Me',
    dj: 'DJ',
    rename: 'Rename',
    statusDJ: 'DJ Status',
    statusOnline: 'Online',
    live: 'LIVE',
    streamListeners: 'Stream listeners {count}'
  },
  namePrompt: {
    title: 'Identification Required',
    descriptionLine1: 'Pick a name before you start interacting.',
    descriptionLine2: 'You can rename yourself later from the user list.',
    placeholder: 'Enter name',
    errors: {
      guestNameReserved: '"Guest" cannot be used as a permanent name'
    }
  },
  chat: {
    title: 'Chat',
    messages: 'Messages',
    tabChat: 'Chat',
    tabSystem: 'System',
    empty: 'No messages yet',
    loading: 'Loading...',
    placeholder: 'Send a message...',
    send: 'Send',
    loginToChat: 'Login to chat',
    readOnly: 'System messages are read-only',
    tabAria: 'Message type'
  },
  tutorial: {
    header: 'Tutorial // {current}/{total}',
    skip: 'Skip',
    next: 'Next >',
    finish: 'Finish',
    steps: {
      rename: {
        desktop: 'Click here to rename yourself. Press Enter to confirm.',
        mobile: 'Open the member list here to rename yourself.'
      },
      search: 'Use search to find songs. You can also search usernames to inspect platform playlists.',
      like: 'Click the center cover art to like the current song.',
      queue: {
        desktop: 'This is the queue. Hover a song to move it up or remove it.',
        mobile: 'Open the play queue here.'
      },
      pause: 'Pause and resume affect every listener globally. Use it carefully.',
      download: 'Like what you hear? Download the current audio file here.',
      random: 'Shuffle uses a fair-random algorithm so everyone gets an equal chance to be played.',
      chat: 'Open chat here to talk with others or review history. The button can be dragged.',
      source: 'Click the small cover at the bottom to open the song source page.'
    }
  },
  liteMode: {
    status: 'Lite Mode',
    nowPlaying: 'Now Playing',
    roomIdle: 'Room Idle',
    autoLiteWhenInactive: 'Auto-Lite when inactive',
    returnToMainView: 'Return to Main View'
  },
  lyrics: {
    empty: 'No lyrics available',
    toggleAlignment: 'Toggle lyric alignment',
    decreaseFont: 'Decrease lyric size',
    increaseFont: 'Increase lyric size',
    toggleTranslation: 'Toggle translation',
    translation: 'Translation'
  },
  platforms: {
    room: 'Room',
    netease: 'Netease',
    bilibili: 'Bilibili',
    navidrome: 'Navidrome'
  }
};
