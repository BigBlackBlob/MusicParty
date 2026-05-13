<template>
  <div class="flex items-center gap-1">
    <IconButton
      size="md"
      @click="$emit('toggle-like')"
      :title="isLiked ? '取消喜欢' : '喜欢'"
    >
      <Heart
        class="w-5 h-5 transition-colors"
        :class="isLiked ? 'fill-[var(--accent)] text-[var(--accent)]' : 'text-[var(--text-secondary)]'"
      />
    </IconButton>

    <IconButton
      size="md"
      @click="$emit('download')"
      title="下载歌曲"
    >
      <Download class="w-5 h-5" />
    </IconButton>

    <IconButton
      size="md"
      @click="$emit('open-source')"
      title="来源页面"
    >
      <ExternalLink class="w-5 h-5" />
    </IconButton>
  </div>
</template>

<script setup>
import { Heart, Download, ExternalLink } from 'lucide-vue-next';
import IconButton from './IconButton.vue';

defineProps({
  isLiked: Boolean
});

defineEmits(['toggle-like', 'download', 'open-source']);
</script>
