<script setup lang="ts">
withDefaults(defineProps<{
  variant?: 'knowledge' | 'agent' | 'shared'
}>(), {
  variant: 'knowledge',
})
</script>

<template>
  <svg
    class="keystone-empty-art"
    viewBox="0 0 200 160"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
    focusable="false"
  >
    <path class="orbit orbit-back" d="M26 88C46 43 132 26 174 67C202 94 155 133 93 132C45 132 7 118 26 88Z" />
    <path class="orbit orbit-front" d="M20 73C49 107 126 126 174 91C203 69 162 40 105 39C61 38 5 55 20 73Z" />

    <g class="network network-back">
      <path d="M31 77L62 55L95 67L128 42L169 69" />
      <path d="M39 112L70 96L103 120L142 104L169 69" />
      <path d="M62 55L70 96L128 42L142 104" />
      <path d="M31 77L95 67L103 120" />
    </g>

    <g class="satellites">
      <g class="satellite satellite-a">
        <circle class="node-halo" cx="31" cy="77" r="7" />
        <circle class="node node-primary" cx="31" cy="77" r="3.5" />
      </g>
      <g class="satellite satellite-b">
        <circle class="node-halo" cx="62" cy="55" r="6" />
        <circle class="node node-secondary" cx="62" cy="55" r="3" />
      </g>
      <g class="satellite satellite-c">
        <circle class="node-halo" cx="128" cy="42" r="6" />
        <circle class="node node-accent" cx="128" cy="42" r="3" />
      </g>
      <g class="satellite satellite-d">
        <circle class="node-halo" cx="169" cy="69" r="7" />
        <circle class="node node-secondary" cx="169" cy="69" r="3.5" />
      </g>
      <g class="satellite satellite-e">
        <circle class="node-halo" cx="142" cy="104" r="6" />
        <circle class="node node-accent" cx="142" cy="104" r="3" />
      </g>
      <g class="satellite satellite-f">
        <circle class="node-halo" cx="103" cy="120" r="6" />
        <circle class="node node-primary" cx="103" cy="120" r="3" />
      </g>
      <g class="satellite satellite-g">
        <circle class="node-halo" cx="39" cy="112" r="5" />
        <circle class="node node-accent" cx="39" cy="112" r="2.5" />
      </g>
    </g>

    <g class="core">
      <path class="core-shadow" d="M75 57L100 43L125 57V87C125 101 114 111 100 117C86 111 75 101 75 87V57Z" />
      <path class="core-shell" d="M77 55L100 42L123 55V85C123 98 113 107 100 113C87 107 77 98 77 85V55Z" />
      <path class="core-inset" d="M84 60L100 51L116 60V83C116 93 109 100 100 105C91 100 84 93 84 83V60Z" />

      <g v-if="variant === 'knowledge'" class="core-symbol knowledge-symbol">
        <path d="M88 66.5C92.7 65.8 96.7 67 100 70V91.5C96.7 88.8 92.7 87.8 88 88.5V66.5Z" />
        <path d="M112 66.5C107.3 65.8 103.3 67 100 70V91.5C103.3 88.8 107.3 87.8 112 88.5V66.5Z" />
        <path class="symbol-detail" d="M92 72.5H96M104 72.5H108M92 78H96M104 78H108" />
      </g>

      <g v-else-if="variant === 'agent'" class="core-symbol agent-symbol">
        <rect x="88" y="68" width="24" height="21" rx="6" />
        <path d="M100 63V68M84.5 77H88M112 77H115.5M94 92V88.5M106 92V88.5" />
        <circle class="symbol-detail-fill" cx="95" cy="77" r="2" />
        <circle class="symbol-detail-fill" cx="105" cy="77" r="2" />
        <path class="symbol-detail" d="M95 83C98 85 102 85 105 83" />
      </g>

      <g v-else class="core-symbol shared-symbol">
        <circle cx="100" cy="71" r="5" />
        <circle cx="90.5" cy="78.5" r="4" />
        <circle cx="109.5" cy="78.5" r="4" />
        <path d="M92 91C92 85.7 95.6 82 100 82C104.4 82 108 85.7 108 91" />
        <path class="symbol-detail" d="M83.5 92C83.5 87.5 86.5 84.5 90.5 84.5M116.5 92C116.5 87.5 113.5 84.5 109.5 84.5" />
      </g>
    </g>

    <g class="sparkles">
      <path d="M53 35V43M49 39H57" />
      <path d="M153 115V123M149 119H157" />
      <circle cx="159" cy="46" r="1.5" />
      <circle cx="64" cy="124" r="1.5" />
    </g>
  </svg>
</template>

<style scoped lang="less">
.keystone-empty-art {
  --keystone-art-primary: var(--td-brand-color, #5b5bd6);
  --keystone-art-secondary: #3b82f6;
  --keystone-art-accent: #8b5cf6;
  display: block;
  width: min(100%, 190px);
  height: auto;
  overflow: visible;
  color: var(--keystone-art-primary);
  filter: drop-shadow(0 18px 24px color-mix(in srgb, var(--keystone-art-primary) 14%, transparent));
}

.orbit {
  stroke: var(--keystone-art-primary);
  stroke-width: 1;
  stroke-linecap: round;
  opacity: 0.18;
  stroke-dasharray: 3 7;
  animation: orbit-flow 8s linear infinite;
}

.orbit-front {
  stroke: var(--keystone-art-secondary);
  animation-duration: 11s;
  animation-direction: reverse;
}

.network {
  stroke: color-mix(in srgb, var(--keystone-art-primary) 36%, transparent);
  stroke-width: 1;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.node-halo {
  fill: color-mix(in srgb, var(--keystone-art-primary) 12%, transparent);
  animation: halo-pulse 3.8s ease-in-out infinite;
}

.node {
  stroke: var(--td-bg-color-container, #fff);
  stroke-width: 1.5;
}

.node-primary {
  fill: var(--keystone-art-primary);
}

.node-secondary {
  fill: var(--keystone-art-secondary);
}

.node-accent {
  fill: var(--keystone-art-accent);
}

.satellite {
  transform-box: fill-box;
  transform-origin: center;
  animation: satellite-drift 4.6s ease-in-out infinite;
}

.satellite-b,
.satellite-e {
  animation-delay: -1.4s;
}

.satellite-c,
.satellite-g {
  animation-delay: -2.7s;
}

.satellite-d,
.satellite-f {
  animation-delay: -3.5s;
}

.core {
  transform-box: fill-box;
  transform-origin: center;
  animation: core-float 4.8s ease-in-out infinite;
}

.core-shadow {
  fill: color-mix(in srgb, var(--keystone-art-primary) 15%, transparent);
}

.core-shell {
  fill: var(--td-bg-color-container, #fff);
  stroke: color-mix(in srgb, var(--keystone-art-primary) 72%, transparent);
  stroke-width: 1.5;
}

.core-inset {
  fill: color-mix(in srgb, var(--keystone-art-primary) 10%, var(--td-bg-color-container, #fff));
  stroke: color-mix(in srgb, var(--keystone-art-primary) 22%, transparent);
  stroke-width: 1;
}

.core-symbol {
  color: var(--keystone-art-primary);
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.knowledge-symbol path,
.agent-symbol rect,
.shared-symbol circle,
.shared-symbol > path:not(.symbol-detail) {
  fill: color-mix(in srgb, var(--keystone-art-primary) 12%, var(--td-bg-color-container, #fff));
}

.symbol-detail {
  fill: none !important;
  stroke: var(--keystone-art-secondary);
  stroke-width: 1.35;
}

.symbol-detail-fill {
  fill: var(--keystone-art-secondary);
  stroke: none;
}

.sparkles {
  stroke: var(--keystone-art-accent);
  stroke-width: 1.4;
  stroke-linecap: round;
  opacity: 0.62;
  animation: sparkle 3.2s ease-in-out infinite;
}

@keyframes orbit-flow {
  to {
    stroke-dashoffset: -40;
  }
}

@keyframes halo-pulse {
  0%, 100% {
    opacity: 0.45;
    transform: scale(0.82);
  }
  50% {
    opacity: 0.9;
    transform: scale(1.08);
  }
}

@keyframes satellite-drift {
  0%, 100% {
    transform: translate3d(0, 0, 0);
  }
  50% {
    transform: translate3d(0, -3px, 0);
  }
}

@keyframes core-float {
  0%, 100% {
    transform: translate3d(0, 0, 0);
  }
  50% {
    transform: translate3d(0, -2px, 0);
  }
}

@keyframes sparkle {
  0%, 100% {
    opacity: 0.35;
  }
  50% {
    opacity: 0.9;
  }
}

@media (prefers-reduced-motion: reduce) {
  .orbit,
  .node-halo,
  .satellite,
  .core,
  .sparkles {
    animation: none;
  }
}
</style>
