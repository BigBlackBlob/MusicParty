import { createApp, h, nextTick, ref } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { afterEach, describe, expect, it, vi } from 'vitest';
import en from '../../i18n/en';
import zh from '../../i18n/zh';
import MobileBottomNav from './MobileBottomNav.vue';
import MobileMembersView from './MobileMembersView.vue';
import { useChatStore } from '../../stores/chat';
import { useUiStore } from '../../stores/ui';
import { useUserStore } from '../../stores/user';
import { useRoomPresenceStore } from '../../domains/realtime/roomPresenceStore';
import { useRoomRuntimeStore } from '../../domains/realtime/roomRuntimeStore';

const mockRenameUser = vi.fn();
vi.mock('../../domains/realtime/roomRealtimeCoordinator', () => ({
  useRoomRealtimeCoordinator: () => ({
    renameUser: mockRenameUser
  })
}));

let app;
let host;

const createTestContext = (locale = 'en') => {
  const pinia = createPinia();
  setActivePinia(pinia);
  const i18n = createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en',
    messages: { en, zh }
  });
  host = document.createElement('div');
  document.body.appendChild(host);
  return { pinia, i18n };
};

afterEach(() => {
  app?.unmount();
  host?.remove();
  localStorage.clear();
  vi.clearAllMocks();
});

describe('MobileBottomNav members tab', () => {
  it('renders a 5th Members tab with live online count badge and emits selection', async () => {
    const { pinia, i18n } = createTestContext('zh');
    const presence = useRoomPresenceStore();
    const chat = useChatStore();
    presence.users = [
      { publicId: 'u1', name: 'Mika', isGuest: false },
      { publicId: 'u2', name: 'Rin', isGuest: false },
      { publicId: 'u3', name: 'Guest 1', isGuest: true }
    ];
    chat.unreadCount = 0;

    const active = ref('now');
    app = createApp(() => h(MobileBottomNav, {
      active: active.value,
      'onUpdate:active': (val) => {
        active.value = val;
      }
    }));
    app.use(pinia);
    app.use(i18n);
    app.mount(host);
    await nextTick();

    const tabs = Array.from(host.querySelectorAll('button[role="tab"]'));
    expect(tabs).toHaveLength(5);

    const membersTab = tabs[4];
    expect(membersTab.getAttribute('aria-label')).toBe('成员');
    expect(membersTab.textContent).toContain('成员');
    expect(membersTab.textContent).toContain('3');
    expect(membersTab.getAttribute('aria-selected')).toBe('false');

    membersTab.click();
    await nextTick();

    expect(active.value).toBe('members');
    expect(membersTab.getAttribute('aria-selected')).toBe('true');

    // Badge updates reactively with online count
    presence.users = [
      ...presence.users,
      { publicId: 'u4', name: 'Ken', isGuest: false }
    ];
    await nextTick();
    expect(membersTab.textContent).toContain('4');
  });

  it('uses English label "Members" in en locale and ui store allows members tab', async () => {
    const { pinia, i18n } = createTestContext('en');
    const ui = useUiStore();
    ui.setMobileActiveTab('members');
    expect(ui.mobileActiveTab).toBe('members');

    app = createApp(() => h(MobileBottomNav, { active: 'members' }));
    app.use(pinia);
    app.use(i18n);
    app.mount(host);
    await nextTick();

    const tabs = Array.from(host.querySelectorAll('button[role="tab"]'));
    expect(tabs[4].getAttribute('aria-label')).toBe('Members');
    expect(tabs[4].getAttribute('aria-selected')).toBe('true');
  });
});

describe('MobileMembersView grouped member list', () => {
  it('groups members into Current DJ, Liked, and Other listeners and supports self rename', async () => {
    const { pinia, i18n } = createTestContext('en');
    const presence = useRoomPresenceStore();
    const runtime = useRoomRuntimeStore();
    const userStore = useUserStore();

    userStore.account = {
      publicId: 'u2',
      username: 'rin',
      displayName: 'Rin',
      role: 'MEMBER',
      guest: false
    };

    presence.users = [
      { publicId: 'u1', name: 'Mika', isGuest: false },
      { publicId: 'u2', name: 'Rin', isGuest: false },
      { publicId: 'u3', name: 'Alex', isGuest: true }
    ];
    runtime.nowPlaying = {
      music: { id: 'm1', name: 'Merry Christmas Mr. Lawrence', artists: ['Ryuichi Sakamoto'], platform: 'netease' },
      enqueuedById: 'u1',
      enqueuedBy: 'Mika',
      likedUserIds: ['u2']
    };

    app = createApp(MobileMembersView);
    app.use(pinia);
    app.use(i18n);
    app.mount(host);
    await nextTick();

    expect(host.querySelector('[data-testid="mobile-members-count"]')?.textContent).toContain('3 online');
    expect(host.querySelector('[data-testid="mobile-members-dj-spotlight"]')?.textContent).toContain('Merry Christmas Mr. Lawrence');

    const djGroup = host.querySelector('[data-testid="member-group-dj"]');
    const likedGroup = host.querySelector('[data-testid="member-group-liked"]');
    const listenersGroup = host.querySelector('[data-testid="member-group-listeners"]');

    expect(djGroup?.textContent).toContain('Mika');
    expect(djGroup?.querySelector('[aria-label="DJ Status"]')).not.toBeNull();
    expect(likedGroup?.querySelector('input[aria-label="Rename"]')).not.toBeNull();
    expect(listenersGroup?.textContent).toContain('Alex');
    expect(listenersGroup?.textContent).toContain('Guest');

    // Rename self triggers realtimeCoordinator.renameUser
    const renameInput = likedGroup?.querySelector('input[aria-label="Rename"]');
    renameInput.value = 'RinRenamed';
    renameInput.dispatchEvent(new Event('input'));
    renameInput.dispatchEvent(new Event('blur'));
    await nextTick();
    expect(mockRenameUser).toHaveBeenCalledWith('RinRenamed');
  });
});
