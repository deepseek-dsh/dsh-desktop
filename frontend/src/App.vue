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
const steps = ref([])
const startupFailed = ref(false)

let pollTimer
let jumpTimer
let prewarmFrame

// prewarm 用隐藏 iframe 预加载 Harness 页面并写入缓存, 跳转时命中缓存几乎秒开。
function prewarm(url) {
  if (prewarmFrame) return
  prewarmFrame = document.createElement('iframe')
  prewarmFrame.setAttribute('aria-hidden', 'true')
  prewarmFrame.style.cssText =
    'position:absolute;width:1px;height:1px;opacity:0;pointer-events:none;border:0;'
  prewarmFrame.src = url
  document.body.appendChild(prewarmFrame)
}

// jumpTo 将当前窗口 WebView 直接跳转到 Harness, 窗口主体即为其界面, 无 iframe 嵌套最流畅。
function jumpTo(href) {
  window.location.href = href
}

// scheduleJump 全部步骤完成后等待 2s 再跳转, 让用户看清启动检查的完成态。
function scheduleJump(href) {
  clearTimeout(jumpTimer)
  jumpTimer = setTimeout(() => jumpTo(href), 2000)
}

async function once(p) {
  try {
    return await p
  } catch (err) {
    errorText.value = String(err)
  }
}

// allStepsDone 判断启动检查是否全部步骤完成(至少有一个步骤才成立)。
function allStepsDone(list) {
  return list.length > 0 && list.every((st) => st.status === 'done')
}

async function start() {
  startupFailed.value = false
  const s = await Start()
  applyStatus(s)
  if (s && shouldContinue(s)) {
    scheduleStatusCheck()
  }
}

function applyStatus(s) {
  if (!s) return
  state.value = s.state
  url.value = s.url || ''
  logPath.value = s.logPath || ''
  errorText.value = s.error || ''
  steps.value = s.steps || []

  const failedStep = steps.value.find((st) => st.status === 'failed')
  if (failedStep) {
    startupFailed.value = true
    if (failedStep.detail) errorText.value = failedStep.detail
  }
  // Harness 就绪即开始预加载, 尽早填充缓存。
  if (s.state === 'ready' && s.url) {
    prewarm(s.url)
  }
  // 只有全部启动步骤完成后才跳转主界面。
  if (s.state === 'ready' && s.url && allStepsDone(steps.value)) {
    scheduleJump(s.url)
  }
}

// shouldContinue 决定是否继续轮询: 启动失败/已停止即停, 就绪但步骤未完成则继续。
function shouldContinue(s) {
  if (!s) return false
  if (s.state === 'failed' || s.state === 'stopped') return false
  if (startupFailed.value) return false
  if (s.state === 'ready') return !allStepsDone(s.steps)
  return true
}

function scheduleStatusCheck() {
  clearTimeout(pollTimer)
  pollTimer = setTimeout(async () => {
    const s = await once(Status())
    applyStatus(s)
    if (shouldContinue(s)) {
      scheduleStatusCheck()
    }
  }, 500)
}

onMounted(async () => {
  platform.value = await once(Platform())

  const s = await once(Status())
  if (s) {
    applyStatus(s)
    if (s.state === 'ready' && allStepsDone(s.steps)) return
  }

  if (state.value === 'idle' || state.value === 'stopped') {
    start()
  } else if (shouldContinue(s)) {
    scheduleStatusCheck()
  }
})

onUnmounted(() => {
  clearTimeout(pollTimer)
  clearTimeout(jumpTimer)
})

const isFailed = () => startupFailed.value || state.value === 'failed'
const isStopped = () => state.value === 'stopped'
</script>

<template>
  <main class="shell">
    <!-- 启动检查: 左侧品牌呼吸, 右侧分步展示启动进度 -->
    <section v-if="!isFailed() && !isStopped()" class="boot">
      <aside class="brand">
        <div class="logo-wrap">
          <span class="glow"></span>
          <img class="bootlogo" src="./assets/images/favicon.svg" alt="DSH Desktop" />
        </div>
        <span class="brand-name">DSH Desktop</span>
        <span class="brand-sub">正在准备 DeepSeek Harness…</span>
      </aside>

      <section class="check">
        <h2>启动检查</h2>
        <ul class="steps">
          <li v-for="s in steps" :key="s.id" :class="s.status">
            <span class="dot"></span>
            <div class="text">
              <span class="title">{{ s.title }}</span>
              <span class="detail" :title="s.detail">{{ s.detail }}</span>
            </div>
          </li>
        </ul>
      </section>
    </section>

    <!-- 启动失败 -->
    <section v-else-if="isFailed()" class="card center">
      <div class="err">⚠</div>
      <h2>启动失败</h2>
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
  --bg: #f6f7f9;
  --surface: #ffffff;
  --border: #e4e7ec;
  --ink: #101828;
  --ink-2: #667085;
  --brand: #4d6bfe;
  --ok: #12b76a;
  --warn: #f79009;
  --danger: #f04438;
  font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", "PingFang SC",
    "Microsoft YaHei", sans-serif;
  color: var(--ink);
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, var(--surface) 0%, var(--bg) 100%);
}
.boot {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 96px;
}
.brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
}
.logo-wrap {
  position: relative;
  width: 148px;
  height: 148px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.glow {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: radial-gradient(
    circle,
    rgba(77, 107, 254, 0.32) 0%,
    rgba(77, 107, 254, 0) 70%
  );
  animation: glowBreathe 2.2s ease-in-out infinite;
}
.bootlogo {
  position: relative;
  width: 132px;
  height: 132px;
  object-fit: contain;
  animation: bootBreathe 2.2s ease-in-out infinite;
  filter: drop-shadow(0 10px 28px rgba(77, 107, 254, 0.18));
}
.brand-name {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--ink);
}
.brand-sub {
  font-size: 12px;
  color: var(--ink-2);
}
@keyframes bootBreathe {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.04); }
}
@keyframes glowBreathe {
  0%, 100% { opacity: 0.3; transform: scale(0.92); }
  50% { opacity: 1; transform: scale(1.08); }
}
.check {
  width: 400px;
  padding: 28px 28px;
  border-radius: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: 0 1px 2px rgba(16, 24, 40, 0.04), 0 16px 40px rgba(16, 24, 40, 0.06);
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.check h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--ink);
}
.steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.steps li {
  display: grid;
  grid-template-columns: 12px 1fr;
  column-gap: 12px;
  opacity: 0.4;
  transition: opacity 0.3s;
}
.steps li.running,
.steps li.done,
.steps li.failed {
  opacity: 1;
}
.dot {
  grid-column: 1;
  width: 12px;
  height: 12px;
  margin-top: 4px;
  border-radius: 50%;
  background: #d0d5dd;
}
.steps li.running .dot {
  background: var(--warn);
  animation: dotPulse 0.9s ease-in-out infinite;
}
.steps li.done .dot {
  background: var(--ok);
}
.steps li.failed .dot {
  background: var(--danger);
}
@keyframes dotPulse {
  0%, 100% { opacity: 0.45; transform: scale(0.8); }
  50% { opacity: 1; transform: scale(1); }
}
.text {
  grid-column: 2;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.title {
  font-size: 14px;
  line-height: 20px;
  font-weight: 600;
  color: var(--ink);
}
.detail {
  font-size: 12px;
  line-height: 16px;
  min-height: 16px;
  color: var(--ink-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.steps li.failed .title {
  color: var(--danger);
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
.err { color: var(--danger); font-size: 30px; }
.error {
  color: var(--danger);
  max-width: 560px;
  font-size: 14px;
  word-break: break-word;
  white-space: pre-wrap;
}
button {
  cursor: pointer;
  padding: 10px 20px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--ink);
  font-size: 14px;
}
button:hover { background: var(--bg); }
button.primary { background: var(--brand); border-color: var(--brand); color: #ffffff; font-weight: 600; }
button.primary:hover { background: #6b83ff; }
</style>