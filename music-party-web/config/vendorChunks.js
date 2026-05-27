export const vendorChunks = [
  { name: 'vendor-vue', packages: ['vue', 'pinia', 'vue-i18n', '@vueuse/core', 'reka-ui', 'lucide-vue-next'] },
  { name: 'vendor-network', packages: ['axios'] },
  { name: 'vendor-dnd', packages: ['sortablejs'] },
  { name: 'vendor-utils', packages: ['dayjs', 'clsx', 'tailwind-merge', 'class-variance-authority'] }
];

export const normalizeModuleId = (id) => id.replaceAll('\\', '/');
const packagePattern = (pkg) => `/node_modules/${pkg}/`;

export const resolveVendorChunk = (id) => {
  const normalized = normalizeModuleId(id);
  if (!normalized.includes('/node_modules/')) return undefined;

  return vendorChunks.find(({ packages }) =>
    packages.some((pkg) => normalized.includes(packagePattern(pkg)))
  )?.name;
};
