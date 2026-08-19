<script setup>
import {ref, onMounted, onUnmounted} from 'vue'
import {
  Start,
  Status,
  Platform,
} from '../wailsjs/go/app/App'

const state = ref('idle')
const url = ref('')
const logPath = ref('')
const errorText = ref('')
const platform = ref('')

let pollTimer

// jumpTo 将当前窗口 WebView 直接跳转到 Harness, 窗口主体即为其界面, 无 iframe 嵌套最流畅。
function jumpTo(href) {
  window.location.href = href
}

async function once(p) {
  try {
    return await p
  } catch (err) {
    errorText.value = String(err)
  }
}

async function start() {
  const s = await Start()
  applyStatus(s)
  if (s && s.state !== 'ready' && s.state !== 'failed') {
    scheduleStatusCheck()
  }
}

function applyStatus(s) {
  if (!s) return
  state.value = s.state
  url.value = s.url || ''
  logPath.value = s.logPath || ''
  errorText.value = s.error || ''
  if (s.state === 'ready' && s.url) {
    jumpTo(s.url)
  }
}

function scheduleStatusCheck() {
  clearTimeout(pollTimer)
  pollTimer = setTimeout(async () => {
    const s = await once(Status())
    applyStatus(s)
    if (s && s.state !== 'ready' && s.state !== 'failed' && s.state !== 'stopped') {
      scheduleStatusCheck()
    }
  }, 500)
}

onMounted(async () => {
  platform.value = await once(Platform())

  const s = await once(Status())
  if (s) {
    applyStatus(s)
    if (s.state === 'ready') return
  }

  if (state.value === 'idle' || state.value === 'stopped') {
    start()
  } else if (state.value === 'starting' || state.value === 'stopping') {
    scheduleStatusCheck()
  }
})

onUnmounted(() => {
  clearTimeout(pollTimer)
})

const isFailed = () => state.value === 'failed'
const isStopped = () => state.value === 'stopped'
</script>

<template>
  <main class="shell" :class="{ booting: !isFailed() && !isStopped() }">
    <!-- 启动中: 透明背景 + favicon 图标动画, 就绪后由状态检查跳转 Harness -->
    <section v-if="!isFailed() && !isStopped()" class="bootbox">
      <img class="bootlogo" src="./assets/images/favicon.svg" alt="DSH Desktop" />
    </section>

    <!-- 启动失败 -->
    <section v-else-if="isFailed()" class="card center">
      <div class="err">⚠</div>
      <h2>Harness 启动失败</h2>
      <p class="error">{{ errorText }}</p>
      <button class="primary" @click="start">重试</button>
    </section>

    <!-- 已停止 -->
    <section v-else-if="isStopped()" class="card center">
      <h2>Harness 已停止</h2>
      <button class="primary" @click="start">启动</button>
    </section>
  </main>
</template>

<style scoped>
.shell {
  font-family: system-ui, sans-serif;
  color: #eef2f7;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #1b2636, #0f1724);
}
.shell.booting {
  background: transparent;
}
.bootbox {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: transparent;
}
.bootlogo {
  width: 128px;
  height: 128px;
  object-fit: contain;
  animation: bootPulse 1.5s ease-in-out infinite;
  filter: drop-shadow(0 6px 18px rgba(0, 0, 0, 0.18));
}
@keyframes bootPulse {
  0%, 100% { transform: scale(1); opacity: 0.9; }
  50% { transform: scale(1.1); opacity: 1; }
}
.card {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  text-align: center;
  padding: 24px;
}
h2 { margin: 0; font-size: 20px; font-weight: 600; }
.err { color: #ff5c5c; font-size: 30px; }
.error {
  color: #ff8a8a;
  max-width: 560px;
  font-size: 14px;
  word-break: break-word;
  white-space: pre-wrap;
}
button {
  cursor: pointer;
  padding: 10px 20px;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.14);
  background: rgba(255,255,255,0.06);
  color: inherit;
  font-size: 14px;
}
button:hover { background: rgba(255,255,255,0.12); }
button.primary { background: #2f6bff; border-color: #2f6bff; font-weight: 600; }
button.primary:hover { background: #3f78ff; }
</style>
