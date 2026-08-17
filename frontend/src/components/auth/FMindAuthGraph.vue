<template>
  <div ref="rootRef" class="fmind-auth-graph" aria-hidden="true">
    <canvas ref="canvasRef" class="fmind-auth-graph__canvas" role="presentation"></canvas>
    <div class="fmind-auth-graph__grid"></div>
    <div class="fmind-auth-graph__aurora fmind-auth-graph__aurora--one"></div>
    <div class="fmind-auth-graph__aurora fmind-auth-graph__aurora--two"></div>
    <div ref="tooltipRef" class="fmind-auth-graph__tooltip">
      <strong>{{ tooltipTitle }}</strong>
      <span>{{ tooltipKind }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

interface GraphNode {
  name: string
  kind: string
  x: number
  y: number
  radius: number
  vx: number
  vy: number
  phase: number
  drift: number
  depth: number
}

interface GraphLink {
  from: number
  to: number
}

interface GraphPulse extends GraphLink {
  progress: number
  speed: number
}

interface DustParticle {
  x: number
  y: number
  radius: number
  speed: number
  alpha: number
  phase: number
}

const rootRef = ref<HTMLElement>()
const canvasRef = ref<HTMLCanvasElement>()
const tooltipRef = ref<HTMLElement>()
const tooltipTitle = ref('')
const tooltipKind = ref('')

const labels = [
  ['FMind Core', 'Knowledge core'],
  ['RAG', 'Retrieval'],
  ['Wiki', 'Knowledge page'],
  ['Agent', 'Intelligent agent'],
  ['Vector Index', 'Semantic index'],
  ['Document', 'Knowledge source'],
  ['Workflow', 'Orchestration'],
  ['API', 'Integration'],
  ['Model', 'Inference'],
  ['Graph', 'Entity relation'],
  ['Parser', 'Document pipeline'],
  ['Search', 'Hybrid search'],
  ['Workspace', 'Collaboration'],
  ['Policy', 'Access control'],
  ['Embedding', 'Vectorization'],
  ['Citation', 'Evidence'],
  ['Memory', 'Context'],
  ['Dataset', 'Knowledge source'],
  ['Analysis', 'Insight'],
  ['Entity', 'Graph node'],
  ['Question', 'Conversation'],
  ['Archive', 'Knowledge source'],
  ['Insight', 'Analysis'],
  ['Answer', 'Generated result'],
] as const

let animationFrame = 0
let resizeObserver: ResizeObserver | undefined
let themeObserver: MutationObserver | undefined
let reducedMotion: MediaQueryList | undefined
let context: CanvasRenderingContext2D | null = null
let width = 0
let height = 0
let dpr = 1
let randomState = 0x4b455953
let nodes: GraphNode[] = []
let links: GraphLink[] = []
let dust: DustParticle[] = []
let pulses: GraphPulse[] = []
let hoverIndex = -1
let selectedIndex = 0
let draggingIndex = -1
let panning = false
let lastPointer = { x: 0, y: 0 }
let lastPulseAt = 0
let isDark = false

const view = { x: 0, y: 0, zoom: 1 }
const target = { x: 0, y: 0, zoom: 1 }

const random = () => {
  randomState = (randomState * 1664525 + 1013904223) >>> 0
  return randomState / 4294967296
}

const graphHomeX = () => width * (width < 760 ? 0.5 : 0.28)

const resetGraph = () => {
  randomState = 0x4b455953
  nodes = labels.map(([name, kind], index) => {
    const angle = (index / labels.length) * Math.PI * 2 + (random() - 0.5) * 0.28
    const distance = 128 + random() * 340
    return {
      name,
      kind,
      x: index === 0 ? 0 : Math.cos(angle) * distance,
      y: index === 0 ? 0 : Math.sin(angle) * distance * 0.58,
      radius: index === 0 ? 14 : 3.5 + random() * 3.8,
      vx: 0,
      vy: 0,
      phase: random() * Math.PI * 2,
      drift: index === 0 ? 0 : 0.35 + random() * 0.8,
      depth: index === 0 ? 0 : 0.3 + random() * 0.7,
    }
  })

  links = []
  for (let index = 1; index < nodes.length; index += 1) {
    links.push({ from: index, to: Math.floor(random() * index) })
    if (random() > 0.44) links.push({ from: index, to: Math.floor(random() * index) })
  }
  for (let index = 0; index < 15; index += 1) {
    const from = Math.floor(random() * nodes.length)
    const to = Math.floor(random() * nodes.length)
    if (from !== to) links.push({ from, to })
  }

  dust = Array.from({ length: 78 }, () => ({
    x: random(),
    y: random(),
    radius: 0.35 + random() * 1.15,
    speed: 0.12 + random() * 0.38,
    alpha: 0.11 + random() * 0.3,
    phase: random() * 7,
  }))
  pulses = []
}

const syncTheme = () => {
  isDark = document.documentElement.getAttribute('theme-mode') === 'dark'
}

const resizeCanvas = () => {
  const root = rootRef.value
  const canvas = canvasRef.value
  if (!root || !canvas) return

  const bounds = root.getBoundingClientRect()
  width = Math.max(1, bounds.width)
  height = Math.max(1, bounds.height)
  dpr = Math.min(window.devicePixelRatio || 1, 2)
  canvas.width = Math.round(width * dpr)
  canvas.height = Math.round(height * dpr)
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`
  context = canvas.getContext('2d')
  context?.setTransform(dpr, 0, 0, dpr, 0, 0)

  const nextX = graphHomeX()
  if (!view.x && !view.y) {
    view.x = nextX
    view.y = height * 0.5
    target.x = view.x
    target.y = view.y
  }
}

const toScreen = (node: GraphNode) => ({
  x: view.x + node.x * view.zoom,
  y: view.y + node.y * view.zoom,
})

const toWorld = (point: { x: number; y: number }) => ({
  x: (point.x - view.x) / view.zoom,
  y: (point.y - view.y) / view.zoom,
})

const localPointer = (event: PointerEvent | WheelEvent) => {
  const bounds = rootRef.value?.getBoundingClientRect()
  return {
    x: event.clientX - (bounds?.left || 0),
    y: event.clientY - (bounds?.top || 0),
  }
}

const hitTest = (point: { x: number; y: number }) => {
  let match = -1
  let distance = 24
  nodes.forEach((node, index) => {
    const screen = toScreen(node)
    const nextDistance = Math.hypot(point.x - screen.x, point.y - screen.y)
    if (nextDistance < (node.radius + 9) * view.zoom && nextDistance < distance) {
      match = index
      distance = nextDistance
    }
  })
  return match
}

const hideTooltip = () => {
  if (tooltipRef.value) tooltipRef.value.style.visibility = 'hidden'
}

const handlePointerMove = (event: PointerEvent) => {
  const point = localPointer(event)
  if (draggingIndex >= 0) {
    const world = toWorld(point)
    const node = nodes[draggingIndex]
    node.x = world.x
    node.y = world.y
    node.vx = 0
    node.vy = 0
    return
  }
  if (panning) {
    target.x += point.x - lastPointer.x
    target.y += point.y - lastPointer.y
    lastPointer = point
    return
  }

  hoverIndex = hitTest(point)
  const tooltip = tooltipRef.value
  if (hoverIndex >= 0 && tooltip) {
    const node = nodes[hoverIndex]
    tooltipTitle.value = node.name
    tooltipKind.value = node.kind
    tooltip.style.left = `${Math.min(width - 180, point.x + 16)}px`
    tooltip.style.top = `${Math.min(height - 72, point.y + 16)}px`
    tooltip.style.visibility = 'visible'
  } else {
    hideTooltip()
  }
}

const handlePointerDown = (event: PointerEvent) => {
  const canvas = canvasRef.value
  if (!canvas) return
  const point = localPointer(event)
  lastPointer = point
  const hit = hitTest(point)
  if (hit >= 0) {
    draggingIndex = hit
    selectedIndex = hit
    hideTooltip()
  } else {
    panning = true
  }
  canvas.setPointerCapture(event.pointerId)
}

const handlePointerUp = (event: PointerEvent) => {
  const canvas = canvasRef.value
  if (draggingIndex >= 0) selectedIndex = draggingIndex
  draggingIndex = -1
  panning = false
  if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
}

const handleWheel = (event: WheelEvent) => {
  event.preventDefault()
  const point = localPointer(event)
  const before = toWorld(point)
  const zoom = Math.max(0.48, Math.min(2.1, target.zoom * (event.deltaY < 0 ? 1.1 : 0.9)))
  target.zoom = zoom
  target.x = point.x - before.x * zoom
  target.y = point.y - before.y * zoom
}

const updatePhysics = (time: number) => {
  if (reducedMotion?.matches) return
  for (let index = 0; index < nodes.length; index += 1) {
    const node = nodes[index]
    if (index === 0) {
      node.x = 0
      node.y = 0
      node.vx = 0
      node.vy = 0
      continue
    }
    if (index === draggingIndex) continue

    const wave = time * 0.00022 * node.drift + node.phase
    node.vx = (-node.x * 0.00005 + Math.cos(wave) * 0.01 * node.depth + node.vx) * 0.955
    node.vy = (-node.y * 0.00005 + Math.sin(wave * 1.17) * 0.008 * node.depth + node.vy) * 0.955

    for (let otherIndex = index + 1; otherIndex < nodes.length; otherIndex += 1) {
      const other = nodes[otherIndex]
      const dx = node.x - other.x
      const dy = node.y - other.y
      const force = 72 / (dx * dx + dy * dy + 90)
      node.vx += dx * force * 0.004
      node.vy += dy * force * 0.004
      other.vx -= dx * force * 0.004
      other.vy -= dy * force * 0.004
    }

    node.x += node.vx
    node.y += node.vy
  }
}

const drawLine = (
  start: { x: number; y: number },
  end: { x: number; y: number },
  alpha: number,
  lineWidth = 1,
) => {
  if (!context) return
  const color = isDark ? '129,140,248' : '52,71,255'
  context.beginPath()
  context.moveTo(start.x, start.y)
  context.lineTo(end.x, end.y)
  context.strokeStyle = `rgba(${color},${alpha})`
  context.lineWidth = lineWidth
  context.stroke()
}

const render = (time: number) => {
  if (!context) {
    animationFrame = requestAnimationFrame(render)
    return
  }

  updatePhysics(time)
  view.x += (target.x - view.x) * 0.075
  view.y += (target.y - view.y) * 0.075
  view.zoom += (target.zoom - view.zoom) * 0.09
  context.clearRect(0, 0, width, height)

  const primary = isDark ? '129,140,248' : '52,71,255'
  const secondary = isDark ? '167,139,250' : '91,91,214'
  const textColor = isDark ? '#dbe2ff' : '#28304a'
  const ambient = context.createRadialGradient(view.x, view.y, 20, view.x, view.y, Math.max(width, height) * 0.62)
  ambient.addColorStop(0, `rgba(${primary},${isDark ? 0.2 : 0.14})`)
  ambient.addColorStop(0.34, `rgba(${secondary},${isDark ? 0.1 : 0.07})`)
  ambient.addColorStop(1, `rgba(${primary},0)`)
  context.fillStyle = ambient
  context.fillRect(0, 0, width, height)

  dust.forEach((particle, index) => {
    const x = (particle.x * width + time * particle.speed * 0.014) % width
    const y = particle.y * height + Math.sin(time * 0.00055 + particle.phase) * 13 * particle.speed
    const blink = 0.45 + 0.55 * Math.sin(time * 0.0014 + particle.phase)
    context?.beginPath()
    context?.arc(x, y, particle.radius * (0.8 + blink * 0.35), 0, Math.PI * 2)
    if (context) context.fillStyle = `rgba(${primary},${particle.alpha * (0.5 + blink * 0.5)})`
    context?.fill()
    if (index % 17 === 0 && context) {
      context.beginPath()
      context.moveTo(x - 5, y)
      context.lineTo(x + 5, y)
      context.moveTo(x, y - 5)
      context.lineTo(x, y + 5)
      context.strokeStyle = `rgba(${primary},${0.1 + blink * 0.13})`
      context.stroke()
    }
  })

  const core = toScreen(nodes[0])
  for (let ring = 0; ring < 6; ring += 1) {
    context.save()
    context.translate(core.x, core.y)
    context.rotate(time * (ring % 2 ? -0.000045 : 0.000035) + ring * 0.63)
    context.scale(1, 0.44 + 0.025 * Math.sin(time * 0.0006 + ring))
    context.setLineDash([2 + (ring % 2), 8 + ring * 2])
    context.beginPath()
    context.arc(0, 0, (48 + ring * 32) * view.zoom, 0, Math.PI * 2)
    context.strokeStyle = `rgba(${primary},${0.2 - ring * 0.025})`
    context.lineWidth = ring === 0 ? 1.4 : 0.8
    context.stroke()
    context.restore()
  }

  links.forEach((link, index) => {
    const start = toScreen(nodes[link.from])
    const end = toScreen(nodes[link.to])
    const isActive = link.from === selectedIndex || link.to === selectedIndex || link.from === hoverIndex || link.to === hoverIndex
    const shimmer = 0.84 + 0.16 * Math.sin(time * 0.001 + index)
    drawLine(start, end, isActive ? 0.53 : 0.13 * shimmer, isActive ? 1.25 : 0.65)
  })

  nodes.forEach((node, index) => {
    if (!context) return
    const point = toScreen(node)
    const isActive = index === selectedIndex || index === hoverIndex
    const float = Math.sin(time * 0.001 * node.drift + node.phase)
    const radius = (node.radius + (isActive ? 3 : 0) + float * 0.5) * view.zoom
    const glow = context.createRadialGradient(point.x, point.y, 0, point.x, point.y, radius * 5.6)
    glow.addColorStop(0, `rgba(${primary},${isActive ? 0.28 : 0.13})`)
    glow.addColorStop(1, `rgba(${primary},0)`)
    context.fillStyle = glow
    context.beginPath()
    context.arc(point.x, point.y, radius * 5.6, 0, Math.PI * 2)
    context.fill()
    context.beginPath()
    context.arc(point.x, point.y, radius, 0, Math.PI * 2)
    context.fillStyle = index === 0 ? `rgb(${primary})` : index % 3 === 0 ? `rgb(${secondary})` : isDark ? '#94a3ff' : '#7184ff'
    context.shadowBlur = isActive ? 24 : 11
    context.shadowColor = context.fillStyle
    context.fill()
    context.shadowBlur = 0
    if (isActive || index === 0) {
      context.fillStyle = textColor
      context.font = `${Math.max(10, 11 * view.zoom)}px Inter, "PingFang SC", "Microsoft YaHei", sans-serif`
      context.fillText(node.name, point.x + radius + 8, point.y + 4)
    }
  })

  if (!reducedMotion?.matches && time - lastPulseAt > 420) {
    const related = links.filter(link => link.from === selectedIndex || link.to === selectedIndex)
    const source = related.length ? related : links
    const link = source[Math.floor(random() * source.length)]
    if (link) pulses.push({ ...link, progress: 0, speed: 0.009 + random() * 0.008 })
    lastPulseAt = time
  }

  for (let index = pulses.length - 1; index >= 0; index -= 1) {
    const pulse = pulses[index]
    pulse.progress += pulse.speed
    const start = toScreen(nodes[pulse.from])
    const end = toScreen(nodes[pulse.to])
    const x = start.x + (end.x - start.x) * pulse.progress
    const y = start.y + (end.y - start.y) * pulse.progress
    context.beginPath()
    context.arc(x, y, 2.2, 0, Math.PI * 2)
    context.fillStyle = isDark ? '#ffffff' : '#f8fbff'
    context.shadowBlur = 20
    context.shadowColor = `rgb(${primary})`
    context.fill()
    context.shadowBlur = 0
    if (pulse.progress > 1) pulses.splice(index, 1)
  }

  animationFrame = requestAnimationFrame(render)
}

onMounted(() => {
  resetGraph()
  syncTheme()
  resizeCanvas()

  const canvas = canvasRef.value
  canvas?.addEventListener('pointermove', handlePointerMove)
  canvas?.addEventListener('pointerdown', handlePointerDown)
  canvas?.addEventListener('pointerup', handlePointerUp)
  canvas?.addEventListener('pointercancel', handlePointerUp)
  canvas?.addEventListener('wheel', handleWheel, { passive: false })

  resizeObserver = new ResizeObserver(resizeCanvas)
  if (rootRef.value) resizeObserver.observe(rootRef.value)
  themeObserver = new MutationObserver(syncTheme)
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['theme-mode'] })
  reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
  animationFrame = requestAnimationFrame(render)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(animationFrame)
  resizeObserver?.disconnect()
  themeObserver?.disconnect()
  const canvas = canvasRef.value
  canvas?.removeEventListener('pointermove', handlePointerMove)
  canvas?.removeEventListener('pointerdown', handlePointerDown)
  canvas?.removeEventListener('pointerup', handlePointerUp)
  canvas?.removeEventListener('pointercancel', handlePointerUp)
  canvas?.removeEventListener('wheel', handleWheel)
})
</script>

<style lang="less" scoped>
.fmind-auth-graph {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background:
    radial-gradient(circle at 26% 48%, rgba(63, 81, 255, 0.09), transparent 38%),
    radial-gradient(circle at 76% 22%, rgba(129, 140, 248, 0.08), transparent 30%),
    #fbfcff;
  isolation: isolate;
}

.fmind-auth-graph__canvas,
.fmind-auth-graph__grid,
.fmind-auth-graph__aurora {
  position: absolute;
  inset: 0;
}

.fmind-auth-graph__canvas {
  z-index: 2;
  width: 100%;
  height: 100%;
  cursor: grab;
  touch-action: none;

  &:active {
    cursor: grabbing;
  }
}

.fmind-auth-graph__grid {
  z-index: 1;
  pointer-events: none;
  opacity: 0.72;
  background-image:
    linear-gradient(rgba(52, 71, 255, 0.038) 1px, transparent 1px),
    linear-gradient(90deg, rgba(52, 71, 255, 0.038) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse at 42% 50%, #000 12%, transparent 80%);
}

.fmind-auth-graph__aurora {
  z-index: 0;
  pointer-events: none;
  filter: blur(58px);
  opacity: 0.7;
  will-change: transform;
}

.fmind-auth-graph__aurora--one {
  inset: -44% -18% -36% -26%;
  background: conic-gradient(from 122deg at 48% 46%, transparent 0 18%, rgba(52, 71, 255, 0.14) 28%, transparent 40% 59%, rgba(129, 140, 248, 0.12) 71%, transparent 84%);
  animation: authAuroraSpin 26s linear infinite;
}

.fmind-auth-graph__aurora--two {
  width: 78vw;
  height: 78vw;
  left: -12vw;
  top: -24vw;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(52, 71, 255, 0.14), rgba(91, 91, 214, 0.06) 38%, transparent 70%);
  animation: authAuroraFloat 11s ease-in-out infinite alternate;
}

.fmind-auth-graph__tooltip {
  position: absolute;
  z-index: 4;
  visibility: hidden;
  min-width: 148px;
  padding: 9px 11px;
  pointer-events: none;
  color: #69718a;
  font-size: 11px;
  border: 1px solid #dbe2ff;
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 14px 36px rgba(0, 31, 136, 0.13);
  backdrop-filter: blur(14px);

  strong {
    display: block;
    margin-bottom: 3px;
    color: #21263a;
  }

  span {
    color: #3447ff;
  }
}

@keyframes authAuroraSpin {
  to {
    transform: rotate(360deg) scale(1.08);
  }
}

@keyframes authAuroraFloat {
  to {
    transform: translate(160px, 52px) scale(1.18);
  }
}

@media (prefers-reduced-motion: reduce) {
  .fmind-auth-graph__aurora {
    animation: none;
  }
}
</style>

<style lang="less">
html[theme-mode='dark'] {
  .fmind-auth-graph {
    background:
      radial-gradient(circle at 26% 48%, rgba(99, 102, 241, 0.18), transparent 40%),
      radial-gradient(circle at 76% 22%, rgba(129, 140, 248, 0.12), transparent 32%),
      #0c1020;
  }

  .fmind-auth-graph__grid {
    opacity: 0.58;
    background-image:
      linear-gradient(rgba(129, 140, 248, 0.07) 1px, transparent 1px),
      linear-gradient(90deg, rgba(129, 140, 248, 0.07) 1px, transparent 1px);
  }

  .fmind-auth-graph__tooltip {
    color: #aeb7d0;
    border-color: rgba(129, 140, 248, 0.28);
    background: rgba(18, 23, 45, 0.92);
    box-shadow: 0 14px 36px rgba(0, 0, 0, 0.35);

    strong {
      color: #eef1ff;
    }

    span {
      color: #a5b4fc;
    }
  }
}
</style>
