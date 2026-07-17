<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import KeystoneEmptyStateArt from '@/components/KeystoneEmptyStateArt.vue'
const { t } = useI18n()
</script>
<template>
    <div class="mask">
        <KeystoneEmptyStateArt class="upload-mask-art" variant="knowledge" />
        <span class="drag-txt">{{ $t('file.upload') }}</span>
        <span class="drag-type-txt">{{ $t('knowledgeBase.pdfDocFormat') }}</span>
        <span class="drag-type-txt">{{ $t('knowledgeBase.textMarkdownFormat') }}</span>
    </div>
</template>
<style scoped lang="less">
.mask{
    display: flex;
    flex-flow: column;
    justify-content: center;
    align-items: center;
}
.drag-txt {
    color: var(--td-brand-color);
    font-family: var(--app-font-family);
    font-size: 24px;
    font-weight: 600;
    line-height: 26px;
    display: inline-block;
    margin: 12px 0 16px 0;
}

.drag-type-txt {
    width: 217px;
    color: var(--td-text-color-disabled);
    text-align: center;
    font-family: var(--app-font-family);
    font-size: 12px;
    font-weight: 400;
}
.upload-mask-art {
    width: 210px;
}
</style>
