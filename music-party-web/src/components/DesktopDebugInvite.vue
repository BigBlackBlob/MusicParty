<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { roomApi } from '../api/rooms'
import { httpClient } from '../transport/httpClient'

const local = ['localhost', '127.0.0.1', '[::1]'].includes(window.location.hostname)
const rooms = ref<Awaited<ReturnType<typeof roomApi.list>>>([])
const roomId = ref('')
const code = ref('')
const expiresAt = ref(0)
const busy = ref(false)
const message = ref('')
async function loadRooms() {
  busy.value = true
  try {
    rooms.value = await roomApi.list()
    roomId.value = rooms.value[0]?.roomId ?? ''
    message.value = rooms.value.length ? '' : '暂无可用房间'
  } catch { message.value = '无法读取房间，请确认管理员已登录。' }
  finally { busy.value = false }
}
onMounted(() => { if (local) void loadRooms() })
async function generate() {
  busy.value = true
  code.value = ''
  message.value = ''
  try {
    const result = await httpClient.post<{ code: string; expiresAt: number }, { label: string }>(
      `/api/dev/rooms/${encodeURIComponent(roomId.value)}/invites`, { label: '桌面本机调试' })
    code.value = result.code
    expiresAt.value = result.expiresAt
  } catch { message.value = '生成失败：请确认管理员权限、房间权限及服务已启用非生产调试接口。' }
  finally { busy.value = false }
}
async function copy() {
  try { await navigator.clipboard.writeText(code.value); message.value = '已复制，请在桌面端点击“兑换并进入”。' }
  catch { message.value = '自动复制失败，请选中上方邀请码手工复制。' }
}
</script>

<template>
  <section v-if="local" class="debug-invite" aria-labelledby="debug-invite-title">
    <h3 id="debug-invite-title">桌面调试邀请</h3>
    <div class="invite-row">
      <label for="debug-invite-room">房间</label>
      <select id="debug-invite-room" v-model="roomId" :disabled="busy" @change="code = ''; message = ''">
        <option v-for="room in rooms" :key="room.roomId" :value="room.roomId">{{ room.name }}</option>
      </select>
      <button type="button" :disabled="busy" @click="loadRooms">刷新</button>
    </div>
    <div class="invite-row"><span>仅本地调试</span><button type="button" :disabled="busy || !roomId" @click="generate">{{ busy ? '处理中…' : '生成邀请码' }}</button></div>
    <template v-if="code">
      <div class="invite-row"><input :value="code" readonly aria-label="桌面调试邀请码" @focus="($event.target as HTMLInputElement).select()"><button type="button" @click="copy">复制</button></div>
      <p>{{ expiresAt >= 253402300799000 ? '长期有效，限兑换一次。' : `有效至 ${new Date(expiresAt).toLocaleString()}，限兑换一次。` }}</p>
      <p>在桌面端粘贴邀请码，点击“兑换并进入”。私有房间仍需验证密码。</p>
    </template>
    <p v-if="message" role="status">{{ message }}</p>
  </section>
</template>

<style scoped>
.debug-invite{background:#000;color:#fff;padding:16px 0;border-block:1px solid #444}.debug-invite h3{font-size:16px;margin:0 0 8px}.invite-row{display:flex;align-items:center;gap:12px;padding:10px 0;border-bottom:1px solid #333}.invite-row>span,.invite-row>label{flex:1}.invite-row input,.invite-row select{min-width:0;flex:1;background:#000;color:#fff;border:1px solid #777;padding:7px}.invite-row button{background:#fff;color:#000;border:0;padding:7px 12px;cursor:pointer}.invite-row button:disabled{opacity:.5;cursor:default}.debug-invite p{font-size:12px;margin:8px 0}
</style>
