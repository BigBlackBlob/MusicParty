const DEFAULT_MAX_JSON_LENGTH = 512 * 1024;

export const isPlainObject = (value) =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

export const safeJsonStorage = {
  read(key, fallback, options = {}) {
    const {
      storage = localStorage,
      validate,
      maxLength = DEFAULT_MAX_JSON_LENGTH,
      removeInvalid = true
    } = options;

    const raw = storage.getItem(key);
    if (raw === null) return fallback;

    if (raw.length > maxLength) {
      if (removeInvalid) storage.removeItem(key);
      return fallback;
    }

    try {
      const parsed = JSON.parse(raw);
      if (validate && !validate(parsed)) {
        if (removeInvalid) storage.removeItem(key);
        return fallback;
      }
      return parsed;
    } catch {
      if (removeInvalid) storage.removeItem(key);
      return fallback;
    }
  }
};
