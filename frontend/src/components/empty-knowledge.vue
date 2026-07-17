<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import KeystoneEmptyStateArt from '@/components/KeystoneEmptyStateArt.vue'
const { t } = useI18n()
</script>
<template>
    <div class="empty">
        <KeystoneEmptyStateArt class="empty-art" variant="knowledge" />
        <span class="empty-txt">{{ $t('knowledgeBase.emptyKnowledgeDragDrop') }}</span>
        <span class="empty-type-txt">{{ $t('knowledgeBase.pdfDocFormat') }}</span>
        <span class="empty-type-txt">{{ $t('knowledgeBase.textMarkdownFormat') }}</span>
    </div>
</template>
<style scoped lang="less">
.empty {
    flex: 1;
    display: flex;
    flex-flow: column;
    justify-content: center;
    align-items: center;
}

.empty-txt {
    color: var(--td-text-color-placeholder);
    font-family: var(--app-font-family);
    font-size: 16px;
    font-weight: 600;
    line-height: 26px;
    margin: 12px 0 16px 0;
}

.empty-type-txt {
    color: var(--td-text-color-disabled);
    text-align: center;
    font-family: var(--app-font-family);
    font-size: 12px;
    font-weight: 400;
    width: 217px;
}

.empty-art {
    width: 190px;
}
</style>
