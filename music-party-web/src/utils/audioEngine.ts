export function shouldUseYouTubeEmbed(input: {
  platform: string | undefined
  playerType: string | undefined
  playbackKey: string
  fallbackKey: string
}) {
  return input.playerType === 'youtube-embed' || (
    input.platform === 'youtube' && input.fallbackKey !== '' && input.fallbackKey === input.playbackKey
  )
}
