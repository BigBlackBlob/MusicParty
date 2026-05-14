// src/i18n/index.js
import { createI18n } from 'vue-i18n';
import en from './en';
import zh from './zh';

// Initialize i18n
const locale = localStorage.getItem('mp_locale') || 'en';

export const i18n = createI18n({
  legacy: false, // use Composition API
  locale: locale,
  fallbackLocale: 'en',
  messages: {
    en,
    zh
  }
});
