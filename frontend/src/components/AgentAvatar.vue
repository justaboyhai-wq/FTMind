<template>
  <div 
    class="agent-avatar" 
    :style="avatarStyle"
    :class="{ 'agent-avatar-small': size === 'small', 'agent-avatar-large': size === 'large' }"
  >
    <!-- 星星装饰 - 融入背景 -->
    <svg class="agent-sparkles" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
      <!-- 右上角小星星 -->
      <path d="M24 5L24.4 6.6C24.45 6.85 24.65 7.05 24.9 7.1L26.5 7.5L24.9 7.9C24.65 7.95 24.45 8.15 24.4 8.4L24 10L23.6 8.4C23.55 8.15 23.35 7.95 23.1 7.9L21.5 7.5L23.1 7.1C23.35 7.05 23.55 6.85 23.6 6.6L24 5Z" fill="rgba(255,255,255,0.6)"/>
      <!-- 左下角小星星 -->
      <path d="M7 22L7.4 23.6C7.45 23.85 7.65 24.05 7.9 24.1L9.5 24.5L7.9 24.9C7.65 24.95 7.45 25.15 7.4 25.4L7 27L6.6 25.4C6.55 25.15 6.35 24.95 6.1 24.9L4.5 24.5L6.1 24.1C6.35 24.05 6.55 23.85 6.6 23.6L7 22Z" fill="rgba(255,255,255,0.5)"/>
    </svg>
    <span class="agent-avatar-letter" :style="letterStyle">{{ letter }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = withDefaults(defineProps<{
  name: string;
  size?: 'small' | 'medium' | 'large';
}>(), {
  size: 'medium'
});

// FMind 自有头像色谱：仅使用蓝、靛、紫与少量品红，避免旧品牌绿。
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

// 根据名称生成一个稳定的哈希值
const hashCode = (str: string): number => {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return Math.abs(hash);
};

// 获取首字母（支持中文）
const letter = computed(() => {
  const name = props.name?.trim() || '';
  if (!name) return '?';
  
  // 获取第一个字符
  const firstChar = name.charAt(0);
  
  // 如果是英文字母，转大写
  if (/[a-zA-Z]/.test(firstChar)) {
    return firstChar.toUpperCase();
  }
  
  // 中文或其他字符直接返回
  return firstChar;
});

// 根据名称选择渐变色
const gradient = computed(() => {
  const hash = hashCode(props.name || '');
  return gradients[hash % gradients.length];
});

// 生成样式
const avatarStyle = computed(() => {
  const g = gradient.value;
  return {
    background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)`
  };
});

// 字母样式 - 白色 + 背景色阴影增加层次感
const letterStyle = computed(() => {
  const g = gradient.value;
  return {
    textShadow: `0 1px 2px ${g.to}80, 0 0 8px ${g.from}30`
  };
});
</script>

<style scoped lang="less">
.agent-avatar {
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
  
  &.agent-avatar-small {
    width: 22px;
    height: 22px;
    border-radius: 5px;
    box-shadow: none;
    
    .agent-avatar-letter {
      font-size: 11px;
    }
    
    .agent-sparkles {
      display: none;
    }
  }
  
  &.agent-avatar-large {
    width: 48px;
    height: 48px;
    border-radius: 12px;
    
    .agent-avatar-letter {
      font-size: 20px;
    }
    
    .agent-sparkles {
      opacity: 0.9;
    }
  }
}

.agent-sparkles {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  opacity: 0.85;
}

.agent-avatar-letter {
  position: relative;
  z-index: 1;
  color: var(--td-text-color-anti);
  font-size: 14px;
  font-weight: 600;
  font-family: var(--app-font-family);
}
</style>
