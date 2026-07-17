<template>
  <div
    class="space-avatar"
    :style="avatarStyle"
    :class="{ 'space-avatar-small': size === 'small', 'space-avatar-large': size === 'large', 'space-avatar-emoji': isEmoji }"
  >
    <template v-if="isEmoji">
      <span class="space-avatar-emoji-char">{{ emojiChar }}</span>
    </template>
    <template v-else>
      <svg class="space-avatar-decoration" viewBox="0 0 56 40" preserveAspectRatio="xMaxYMax meet" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
        <circle cx="10" cy="12" r="4" stroke="currentColor" stroke-width="1.5" fill="none" opacity="0.5"/>
        <circle cx="28" cy="8" r="5" stroke="currentColor" stroke-width="1.8" fill="none" opacity="0.7"/>
        <circle cx="46" cy="14" r="4" stroke="currentColor" stroke-width="1.5" fill="none" opacity="0.5"/>
        <path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" opacity="0.4"/>
        <circle cx="28" cy="28" r="6" stroke="currentColor" stroke-width="1.2" fill="none" opacity="0.35"/>
        <path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" stroke-width="1" stroke-linecap="round" opacity="0.3"/>
      </svg>
      <span class="space-avatar-letter" :style="letterStyle">{{ letter }}</span>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = withDefaults(defineProps<{
  name: string;
  /** Optional: "emoji:🚀" for emoji avatar; otherwise name-based */
  avatar?: string;
  size?: 'small' | 'medium' | 'large';
}>(), {
  size: 'medium',
  avatar: ''
});

const isEmoji = computed(() => {
  const v = (props.avatar || '').trim();
  return v.startsWith('emoji:') && v.length > 6;
});

const emojiChar = computed(() => {
  const v = (props.avatar || '').trim();
  if (!v.startsWith('emoji:')) return '';
  return v.slice(6).trim() || '';
});

// Keystone 空间头像色谱：蓝紫知识网络色，不再沿用旧绿色品牌色。
const gradients = [
  { from: '#5b5bd6', to: '#3b82f6' },
  { from: '#4f46e5', to: '#8b5cf6' },
  { from: '#2563eb', to: '#60a5fa' },
  { from: '#3730a3', to: '#6366f1' },
  { from: '#1d4ed8', to: '#7c3aed' },
  { from: '#0f4c81', to: '#4f46e5' },
  { from: '#6366f1', to: '#a855f7' },
  { from: '#1e40af', to: '#4f7cff' },
  { from: '#7c3aed', to: '#ec4899' },
  { from: '#334155', to: '#6366f1' },
  { from: '#3b82f6', to: '#c084fc' },
  { from: '#4338ca', to: '#7e22ce' },
];

const hashCode = (str: string): number => {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return Math.abs(hash);
};

const letter = computed(() => {
  const name = props.name?.trim() || '';
  if (!name) return '?';
  const firstChar = name.charAt(0);
  if (/[a-zA-Z]/.test(firstChar)) return firstChar.toUpperCase();
  return firstChar;
});

const gradient = computed(() => {
  const hash = hashCode(props.name || '');
  return gradients[hash % gradients.length];
});

const avatarStyle = computed(() => {
  if (isEmoji.value) {
    return { background: 'linear-gradient(135deg, #f1f5f9 0%, #e2e8f0 100%)' };
  }
  const g = gradient.value;
  return {
    background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)`
  };
});

const letterStyle = computed(() => {
  const g = gradient.value;
  return {
    textShadow: `0 1px 2px ${g.to}80, 0 0 8px ${g.from}30`
  };
});
</script>

<style scoped lang="less">
.space-avatar {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  flex-shrink: 0;
  box-shadow: var(--td-shadow-2);
  overflow: hidden;

  &.space-avatar-small {
    width: 22px;
    height: 22px;
    border-radius: 5px;
    box-shadow: none;

    .space-avatar-letter {
      font-size: 11px;
    }

    .space-avatar-decoration {
      display: none;
    }
  }

  &.space-avatar-large {
    width: 48px;
    height: 48px;
    border-radius: 12px;

    .space-avatar-letter {
      font-size: 20px;
    }

    .space-avatar-emoji-char {
      font-size: 28px;
    }
  }

  &.space-avatar-emoji {
    .space-avatar-emoji-char {
      position: relative;
      z-index: 1;
      line-height: 1;
      user-select: none;
    }
  }
}

.space-avatar-emoji-char {
  font-size: 18px;
  line-height: 1;

  .space-avatar-small & {
    font-size: 14px;
  }
}

.space-avatar-decoration {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 55%;
  height: 55%;
  opacity: 0.35;
  color: rgba(255, 255, 255, 0.9);
  pointer-events: none;
}

.space-avatar-letter {
  position: relative;
  z-index: 1;
  color: var(--td-text-color-anti);
  font-size: 14px;
  font-weight: 600;
  font-family: var(--app-font-family);
}
</style>
