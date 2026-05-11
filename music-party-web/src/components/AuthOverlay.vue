<template>
  <div v-if="!passed" class="fixed inset-0 z-[200] bg-[var(--surface-0)]/90 backdrop-blur-xl flex items-center justify-center p-4">
    <div class="bg-[var(--surface-4)] p-8 shadow-2xl border border-[var(--border-default)] w-full max-w-md rounded-2xl relative overflow-hidden">
      <div class="absolute inset-x-0 top-0 h-1 bg-[var(--accent)]"></div>

      <div class="mb-6">
        <h2 class="text-2xl font-bold text-[var(--text-primary)] tracking-tight">
          {{ isSetupMode ? 'INITIALIZE SYSTEM' : 'PbSECURITY ACCESS' }}
        </h2>
        <p class="text-xs font-mono text-[var(--text-tertiary)] mt-1 tracking-[0.2em]">
          {{ isSetupMode ? 'PLEASE CONFIGURE ROOM ACCESS.' : 'RESTRICTED AREA. ENTER PASSCODE.' }}
        </p>
      </div>

      <div class="space-y-4">
        <input
            v-if="!isSetupMode || (isSetupMode && setupType === 'password')"
            v-model="inputPassword"
            type="password"
            :placeholder="isSetupMode ? 'SET NEW PASSWORD' : 'INPUT PASSWORD'"
            @keyup.enter="handleAction"
            class="w-full bg-[var(--surface-2)] border border-[var(--border-default)] p-3 outline-none focus:border-[var(--accent)] font-mono text-center tracking-widest text-lg rounded-xl text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)]"
            autofocus
        />

        <button
            v-if="!isSetupMode || setupType === 'password'"
            @click="handleAction"
            :disabled="loading"
            class="w-full bg-[var(--accent)] text-[var(--text-inverse)] font-semibold py-3 hover:bg-[var(--accent-hover)] transition-colors disabled:opacity-50 rounded-xl"
        >
          {{ loading ? 'VERIFYING...' : (isSetupMode ? 'CONFIRM PASSWORD' : 'UNLOCK') }}
        </button>

        <div v-if="isSetupMode && setupType === 'initial'" class="space-y-3">
          <button
              @click="setupType = 'password'"
              class="w-full bg-[var(--surface-2)] text-[var(--text-primary)] font-semibold py-3 hover:bg-[var(--surface-3)] transition-colors rounded-xl border border-[var(--border-default)]"
          >
            SET PASSWORD PROTECTION
          </button>

          <div class="relative flex py-2 items-center">
            <div class="flex-grow border-t border-[var(--border-default)]"></div>
            <span class="flex-shrink-0 mx-4 text-[var(--text-tertiary)] text-xs font-mono">OR</span>
            <div class="flex-grow border-t border-[var(--border-default)]"></div>
          </div>

          <button
              @click="setupNoPassword"
              class="w-full bg-[var(--surface-2)] border border-[var(--border-default)] text-[var(--text-secondary)] font-semibold py-3 hover:bg-[var(--surface-3)] transition-colors hover:text-[var(--text-primary)] rounded-xl"
          >
            NO PASSWORD (PUBLIC)
          </button>
        </div>

        <button
            v-if="isSetupMode && setupType === 'password'"
            @click="setupType = 'initial'"
            class="w-full text-xs text-[var(--text-tertiary)] hover:text-[var(--text-primary)] mt-2 underline"
        >
          &lt; BACK
        </button>

      </div>

      <div v-if="errorMsg" class="mt-4 text-center text-[var(--error-soft-text)] font-mono text-xs animate-pulse">
        > ERROR: {{ errorMsg }}
      </div>
    </div>
  </div>
</template>

<script setup>
import {ref, onMounted} from 'vue';
import {authApi} from '../api/auth';
import {STORAGE_KEYS} from '../constants/keys';

const emit = defineEmits(['unlocked']);

const passed = ref(false);
const isSetupMode = ref(false);
const setupType = ref('initial'); // 'initial' | 'password'
const inputPassword = ref('');
const errorMsg = ref('');
const loading = ref(false);

const checkStatus = async () => {
  loading.value = true;
  try {
    // 🟢 修复点：直接获取数据，不再需要 .data
    // 我们的 api/client.js 里的拦截器已经帮我们把 data 取出来了
    const data = await authApi.getStatus();
    const {isSetup, hasProtection} = data;

    if (!isSetup) {
      isSetupMode.value = true;
    } else {
      if (!hasProtection) {
        passed.value = true;
        emit('unlocked');
      } else {
        const cachedPass = localStorage.getItem(STORAGE_KEYS.ROOM_PASSWORD);
        if (cachedPass) {
          await verify(cachedPass, true);
        }
      }
    }
  } catch (e) {
    console.error("Auth Status Error:", e); // 在控制台打印真实错误
    errorMsg.value = "CONNECTION FAILED";
  } finally {
    loading.value = false;
  }
};

const verify = async (pwd, isAuto = false) => {
  try {
    await authApi.verify(pwd);
    localStorage.setItem(STORAGE_KEYS.ROOM_PASSWORD, pwd);
    passed.value = true;
    emit('unlocked');
  } catch (e) {
    if (!isAuto) {
      errorMsg.value = "INVALID PASSWORD";
      inputPassword.value = '';
    } else {
      localStorage.removeItem(STORAGE_KEYS.ROOM_PASSWORD);
    }
  }
};

const setup = async () => {
  if (!inputPassword.value) {
    errorMsg.value = "PASSWORD CANNOT BE EMPTY";
    return;
  }
  await performSetup(inputPassword.value);
};

const setupNoPassword = async () => {
  await performSetup("");
};

const performSetup = async (pwd) => {
  loading.value = true;
  try {
    await authApi.setup(pwd);
    if (pwd) localStorage.setItem(STORAGE_KEYS.ROOM_PASSWORD, pwd);
    passed.value = true;
    emit('unlocked');
  } catch (e) {
    errorMsg.value = "SETUP FAILED";
    loading.value = false;
  }
};

const handleAction = () => {
  if (loading.value) return;
  errorMsg.value = '';
  if (isSetupMode.value) {
    setup();
  } else {
    verify(inputPassword.value);
  }
};

onMounted(() => {
  checkStatus();
});
</script>
