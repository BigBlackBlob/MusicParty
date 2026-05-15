<template>
  <div class="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-md p-8">
    <div class="w-full max-w-5xl h-[80vh] bg-surface-panel rounded-xl border border-border-default shadow-2xl flex flex-col overflow-hidden">
      <!-- Search Header -->
      <div class="p-6 border-b border-border-default bg-surface-panel">
        <div class="flex items-center justify-between mb-6">
          <div class="flex items-center gap-2">
            <span class="material-symbols-outlined text-primary text-[16px]">search</span>
            <h2 class="text-[10px] font-black uppercase tracking-[0.2em] text-text-tertiary">{{ t('search.discoverMusic') }}</h2>
          </div>
          <button class="text-text-muted hover:text-white transition-colors">
            <span class="material-symbols-outlined">close</span>
          </button>
        </div>

        <div class="flex gap-4">
          <input
            type="text"
            :placeholder="t('search.placeholder')"
            class="flex-1 bg-surface-overlay border border-border-default rounded-[var(--radius-sm)] px-4 py-3 text-sm focus:border-primary outline-none text-text-primary placeholder:text-text-tertiary transition-colors"
          />
          <button class="px-6 bg-primary text-on-primary font-black text-[10px] uppercase tracking-widest rounded-[var(--radius-sm)] hover:bg-primary-fixed-dim transition-colors">
            {{ t('search.title') }}
          </button>
        </div>

        <!-- Search Type Tabs -->
        <div class="flex gap-2 mt-4">
          <button class="px-4 py-1.5 rounded-[var(--radius-sm)] text-[10px] font-black uppercase tracking-widest bg-surface-raised border border-border-default text-primary">{{ t('search.song') }}</button>
          <button class="px-4 py-1.5 rounded-[var(--radius-sm)] text-[10px] font-black uppercase tracking-widest bg-transparent border border-transparent text-text-muted hover:text-text-primary transition-colors">{{ t('search.album') }}</button>
        </div>
      </div>

      <!-- Main Results Area -->
      <div class="flex-1 flex overflow-hidden">
        <!-- Sidebar -->
        <aside class="w-64 border-r border-border-default p-4 overflow-y-auto bg-surface-panel">
          <h3 class="text-[10px] font-black uppercase tracking-widest text-text-tertiary mb-4 px-2">{{ t('search.platforms') }}</h3>
          <div class="space-y-1">
             <button class="w-full text-left px-3 py-2 rounded-[var(--radius-sm)] text-[13px] font-bold bg-surface-raised text-primary">{{ t('platforms.netease') }}</button>
             <button class="w-full text-left px-3 py-2 rounded-[var(--radius-sm)] text-[13px] font-bold text-text-muted hover:bg-surface-raised hover:text-text-primary transition-colors">{{ t('platforms.bilibili') }}</button>
          </div>
        </aside>

        <!-- Results -->
        <main class="flex-1 p-6 overflow-y-auto bg-surface-stage space-y-2">
          <!-- Mock Track Item -->
          <div class="flex items-center gap-4 p-3 rounded-lg hover:bg-white/5 border border-transparent hover:border-border-subtle transition-all cursor-pointer">
            <div class="w-10 h-10 bg-surface-elevated rounded flex items-center justify-center">
              <span class="material-symbols-outlined text-text-muted">music_note</span>
            </div>
            <div class="flex-1">
              <p class="text-text-primary font-compact text-[13px]">Neon Signal</p>
              <p class="text-text-muted text-[11px]">Echo Systems</p>
            </div>
            <span class="text-text-muted font-mono text-[10px]">03:45</span>
          </div>

          <div class="flex items-center gap-4 p-3 rounded-lg hover:bg-white/5 border border-transparent hover:border-border-subtle transition-all cursor-pointer">
            <div class="w-10 h-10 bg-surface-elevated rounded flex items-center justify-center">
              <span class="material-symbols-outlined text-text-muted">music_note</span>
            </div>
            <div class="flex-1">
              <p class="text-text-primary font-compact text-[13px]">Digital Chain</p>
              <p class="text-text-muted text-[11px]">Vapor Wave</p>
            </div>
            <span class="text-text-muted font-mono text-[10px]">05:12</span>
          </div>
        </main>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
</script>
