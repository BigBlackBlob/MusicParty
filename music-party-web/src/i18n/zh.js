export default {
  app: {
    lounge: 'Lounge',
    brand: 'Music Party'
  },
  common: {
    close: '关闭',
    export: '导出',
    unknownArtist: '未知艺术家',
    by: '点歌人',
    cancel: '取消',
    confirm: '确认',
    back: '< 返回',
    or: '或',
    error: '错误'
  },
  auth: {
    initializeTitle: '初始化系统',
    initializeDesc: '请先配置房间访问方式。',
    accessTitle: '安全访问',
    accessDesc: '受限区域，请输入房间密码。',
    setPasswordPlaceholder: '设置新密码',
    inputPasswordPlaceholder: '输入密码',
    verifying: '验证中...',
    confirmPassword: '确认密码',
    unlock: '解锁',
    setPasswordProtection: '启用密码保护',
    noPasswordPublic: '不设密码（公开）',
    errors: {
      connectionFailed: '连接失败',
      invalidPassword: '密码错误',
      passwordEmpty: '密码不能为空',
      setupFailed: '初始化失败'
    }
  },
  settings: {
    title: '设置',
    theme: '主题',
    dark: '深色模式',
    light: '浅色模式',
    language: '语言',
    mobileNowDensity: '播放页密度',
    onlineMembers: '在线成员',
    active: '活跃',
    guest: '访客',
    member: '成员',
    mainSize: '主舞台缩放',
    compact: '紧凑',
    standard: '标准',
    relaxed: '宽松',
    large: '宽大',
    globalZoom: '全局缩放',
    connection: '连接状态',
    connected: '已连接',
    disconnected: '已断开',
    autoLiteMode: '自动精简模式',
    liteMode: '精简模式',
    on: '开启',
    off: '关闭',
    english: '英文',
    chinese: '中文',
    toggleTheme: '切换主题',
    queueAutoHidden: '当前缩放下队列已自动隐藏',
    moreActiveUsers: '还有 {count} 位活跃用户'
  },
  nav: {
    nowPlaying: '播放',
    queue: '队列',
    search: '搜索',
    chat: '聊天'
  },
  player: {
    play: '播放',
    pause: '暂停',
    next: '下一首',
    prev: '上一首',
    shuffle: '随机播放',
    like: '喜欢',
    unlike: '取消喜欢',
    volume: '音量',
    muted: '已静音',
    buffering: '缓冲中',
    loading: '加载中',
    playbackError: '播放错误',
    loadError: '无法加载当前曲目。',
    closeLyrics: '关闭歌词',
    onlyRequester: '仅点歌人可调节进度',
    seek: '调整进度',
    prevUnavailable: '上一首不可用',
    requestedBy: '点歌人 {name}',
    waiting: '等待播放',
    waitingFirstTrack: '等待第一首歌曲',
    nowPlayingIn: '当前所在房间',
    requestedByShort: '点歌',
    listenersShort: '收听'
  },
  queue: {
    title: '播放队列',
    liked: '喜欢的歌',
    upNext: '待播队列',
    tracks: '首',
    selected: '已选',
    all: '全选',
    top: '置顶',
    remove: '移除',
    noLiked: '暂无喜欢的歌曲',
    likedDesc: '你收藏的歌曲会显示在这里。',
    empty: '队列为空',
    emptyDesc: '去搜索添加下一首歌曲吧。',
    clearQueue: '清空队列',
    clearConfirm: '确定要移除队列中的所有歌曲吗？',
    deleteConfirm: '确认删除 {count} 首歌曲？',
    removeAll: '全部移除',
    cancel: '取消',
    loading: '加载中',
    failed: '失败',
    playNext: '下一首播放',
    selectTracks: '选择歌曲',
    exportLiked: '导出喜欢的歌'
  },
  search: {
    placeholder: '搜索歌曲...',
    adminPlaceholder: '输入管理员密码',
    title: '搜索',
    discoverMusic: '发现音乐',
    platforms: '音乐平台',
    noResults: '没有找到歌曲',
    noResultsDesc: '尝试更换平台或关键词。',
    searchAndAdd: '搜索并添加',
    song: '歌曲',
    album: '专辑',
    loading: '正在搜索...',
    noAlbums: '没有找到专辑',
    searchAlbums: '搜索专辑',
    tryDifferent: '换个关键词再试。',
    albumHint: '网易云平台支持按专辑添加。',
    searchSongs: '搜索歌曲',
    startHint: '选择平台后输入关键词。',
    addAlbum: '添加专辑',
    addSong: '添加歌曲',
    searchAria: '搜索',
    platformAria: '平台',
    failed: '搜索失败',
    adminExecuted: '管理员指令已执行',
    adminFailed: '权限不足或指令执行失败'
  },
  playlist: {
    fetchPlaylistsFailed: '获取歌单失败',
    userSearchFailed: '搜索用户失败',
    fetchSongsFailed: '获取歌曲失败'
  },
  userList: {
    me: '我',
    dj: '点歌者',
    rename: '修改昵称',
    statusDJ: '点歌者状态',
    statusOnline: '在线',
    live: '直播',
    streamListeners: '在线收听 {count}'
  },
  namePrompt: {
    title: '需要设置身份',
    descriptionLine1: '操作之前，先给自己取个名字。',
    descriptionLine2: '之后也可以在用户列表里点自己的名字重新命名。',
    placeholder: '输入名字',
    errors: {
      guestNameReserved: '不能使用“游客”作为正式名字'
    }
  },
  chat: {
    title: '聊天',
    messages: '条消息',
    tabChat: '聊天',
    tabSystem: '系统',
    empty: '暂无消息',
    loading: '加载中...',
    placeholder: '发送消息...',
    send: '发送',
    loginToChat: '登录后即可发言',
    readOnly: '系统消息只读',
    tabAria: '消息类型'
  },
  tutorial: {
    header: '新手引导 // {current}/{total}',
    skip: '跳过',
    next: '下一步 >',
    finish: '完成',
    steps: {
      rename: {
        desktop: '点击这里可以修改你的昵称，输入后按回车确认。',
        mobile: '点击这里打开用户列表，可以修改你的昵称。'
      },
      search: '点击搜索按钮寻找歌曲，也可以通过搜索用户名查看平台账号歌单。',
      like: '点击中间的封面可以为当前歌曲点赞。',
      queue: {
        desktop: '这里是播放队列。悬停在歌曲上可以进行置顶或删除操作。',
        mobile: '点击这里查看播放队列。'
      },
      pause: '暂停和播放会全局生效，会影响所有在线听众，请谨慎操作。',
      download: '听到喜欢的歌？点击这里可以直接下载当前播放的音频文件。',
      random: '随机播放模式采用公平随机算法，确保每个人点的歌都有均等的播放机会。',
      chat: '点击浮动按钮打开聊天窗口，可以和其他人聊天或查看记录。按钮可以拖动。',
      source: '点击底部的小封面，可以跳转到歌曲的源网页。'
    }
  },
  liteMode: {
    status: '精简模式',
    nowPlaying: '当前播放',
    roomIdle: '房间空闲',
    autoLiteWhenInactive: '闲置时自动进入精简模式',
    returnToMainView: '返回主视图'
  },
  lyrics: {
    empty: '暂无歌词',
    toggleAlignment: '切换歌词对齐方式',
    decreaseFont: '减小歌词字号',
    increaseFont: '增大歌词字号',
    toggleTranslation: '切换译文显示',
    translation: '译文',
    open: '打开歌词'
  },
  platforms: {
    room: '房间',
    netease: '网易云',
    bilibili: '哔哩哔哩',
    navidrome: 'Navidrome'
  }
};
