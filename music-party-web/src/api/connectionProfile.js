import { STORAGE_KEYS } from '../constants/keys';

const VALID_MODES = new Set(['browser', 'host', 'join']);
const DEFAULT_PROFILE = { mode: 'browser', baseUrl: '' };

let currentProfile = readStoredProfile();
const listeners = new Set();

export function getConnectionProfile() {
  return { ...currentProfile };
}

export function setConnectionProfile(profile = {}) {
  currentProfile = normalizeProfile(profile);
  persistProfile(currentProfile);
  notify();
  return getConnectionProfile();
}

export function onConnectionProfileChange(listener) {
  listeners.add(listener);
  listener(getConnectionProfile());
  return () => listeners.delete(listener);
}

export function resetConnectionProfileForTest() {
  currentProfile = readStoredProfile();
  notify();
}

function notify() {
  const snapshot = getConnectionProfile();
  listeners.forEach(listener => listener(snapshot));
}

function readStoredProfile() {
  if (typeof localStorage === 'undefined') return { ...DEFAULT_PROFILE };
  try {
    return normalizeProfile(JSON.parse(localStorage.getItem(STORAGE_KEYS.CONNECTION_PROFILE) || 'null') || DEFAULT_PROFILE);
  } catch {
    localStorage.removeItem(STORAGE_KEYS.CONNECTION_PROFILE);
    return { ...DEFAULT_PROFILE };
  }
}

function persistProfile(profile) {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(STORAGE_KEYS.CONNECTION_PROFILE, JSON.stringify(profile));
}

function normalizeProfile(profile = {}) {
  const mode = VALID_MODES.has(profile.mode) ? profile.mode : DEFAULT_PROFILE.mode;
  const baseUrl = normalizeBaseUrl(profile.baseUrl);
  return {
    mode: baseUrl ? mode : DEFAULT_PROFILE.mode,
    baseUrl
  };
}

function normalizeBaseUrl(value) {
  if (!value || typeof value !== 'string') return '';
  try {
    const url = new URL(value);
    return url.origin;
  } catch {
    return '';
  }
}
